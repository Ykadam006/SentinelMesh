package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

// ----- Domain Models -----

type Metric struct {
	ServiceID  string  `json:"service_id"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Timestamp  int64   `json:"timestamp"`
}

type Alert struct {
	ServiceID string `json:"service_id"`
	AlertName string `json:"alert_name"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type ServiceStatusChange struct {
	ServiceID string `json:"service_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Timestamp int64  `json:"timestamp"`
	Reason    string `json:"reason"`
}

// ----- SLO Configuration -----

type SLOConfig struct {
	Target           float64 // e.g., 99.9% availability
	WindowDuration   time.Duration
	LatencyThreshold float64 // P95 latency threshold in ms
	ErrorRateMax     float64 // max acceptable error rate %
}

// ----- SLI/SLO Tracking -----

type ServiceSLI struct {
	mu              sync.RWMutex
	TotalRequests   int64
	FailedRequests  int64
	LatencySum      float64
	LatencyCount    int64
	LatencyMax      float64
	LatencyLast     float64
	CurrentStatus   string
	Breaches        map[string]string // metric name -> WARNING/CRITICAL while that metric is over threshold
	LastUpdated     time.Time
	ErrorBudgetUsed float64
}

var (
	sloConfigs = map[string]SLOConfig{
		"payment-service":      {Target: 99.9, WindowDuration: 30 * 24 * time.Hour, LatencyThreshold: 500, ErrorRateMax: 5.0},
		"checkout-service":     {Target: 99.95, WindowDuration: 30 * 24 * time.Hour, LatencyThreshold: 300, ErrorRateMax: 3.0},
		"auth-service":         {Target: 99.99, WindowDuration: 30 * 24 * time.Hour, LatencyThreshold: 200, ErrorRateMax: 1.0},
		"search-service":       {Target: 99.9, WindowDuration: 30 * 24 * time.Hour, LatencyThreshold: 400, ErrorRateMax: 5.0},
		"notification-service": {Target: 99.5, WindowDuration: 30 * 24 * time.Hour, LatencyThreshold: 1000, ErrorRateMax: 10.0},
	}

	serviceSLIs = make(map[string]*ServiceSLI)
	sliMu       sync.RWMutex
)

// ----- Prometheus Custom Metrics -----

var (
	alertsTriggered = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sentinelmesh_alerts_triggered_total",
		Help: "Total number of alerts triggered",
	}, []string{"service_id", "alert_name", "severity"})

	errorBudgetRemaining = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sentinelmesh_error_budget_remaining_percent",
		Help: "Remaining error budget percentage",
	}, []string{"service_id"})

	serviceAvailability = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sentinelmesh_service_availability_percent",
		Help: "Current service availability percentage",
	}, []string{"service_id"})

	metricsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sentinelmesh_metrics_processed_total",
		Help: "Total number of metrics processed by alert engine",
	})
)

func init() {
	prometheus.MustRegister(alertsTriggered, errorBudgetRemaining, serviceAvailability, metricsProcessed)
}

func getKafkaBroker() string {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:29092"
	}
	return broker
}

func getSLI(serviceID string) *ServiceSLI {
	sliMu.Lock()
	defer sliMu.Unlock()
	if sli, ok := serviceSLIs[serviceID]; ok {
		return sli
	}
	sli := &ServiceSLI{CurrentStatus: "HEALTHY", LastUpdated: time.Now(), Breaches: map[string]string{}}
	serviceSLIs[serviceID] = sli
	return sli
}

func calculateAvailability(sli *ServiceSLI) float64 {
	sli.mu.RLock()
	defer sli.mu.RUnlock()
	if sli.TotalRequests == 0 {
		return 100.0
	}
	return (1.0 - float64(sli.FailedRequests)/float64(sli.TotalRequests)) * 100.0
}

func calculateErrorBudgetRemaining(serviceID string, availability float64) float64 {
	config, exists := sloConfigs[serviceID]
	if !exists {
		config = SLOConfig{Target: 99.9}
	}
	maxDowntime := 100.0 - config.Target // e.g., 0.1% for 99.9%
	currentDowntime := 100.0 - availability
	if maxDowntime <= 0 {
		return 0
	}
	remaining := ((maxDowntime - currentDowntime) / maxDowntime) * 100.0
	return math.Max(0, math.Min(100, remaining))
}

func evaluateMetric(metric Metric, alertWriter, statusWriter *kafka.Writer) {
	metricsProcessed.Inc()

	sli := getSLI(metric.ServiceID)
	config, exists := sloConfigs[metric.ServiceID]
	if !exists {
		config = SLOConfig{Target: 99.9, LatencyThreshold: 500, ErrorRateMax: 5.0}
	}

	sli.mu.Lock()

	// Track SLIs
	switch metric.MetricName {
	case "latency":
		sli.LatencySum += metric.Value
		sli.LatencyCount++
		sli.LatencyMax = math.Max(sli.LatencyMax, metric.Value)
		sli.LatencyLast = metric.Value
	case "error_rate":
		sli.TotalRequests += 100 // approximate per batch
		sli.FailedRequests += int64(metric.Value)
	case "request_rate":
		sli.TotalRequests += int64(metric.Value)
	}
	sli.LastUpdated = time.Now()
	sli.mu.Unlock()

	// Evaluate alert conditions
	var alerts []Alert

	if metric.MetricName == "latency" && metric.Value > config.LatencyThreshold {
		severity := "WARNING"
		if metric.Value > config.LatencyThreshold*2 {
			severity = "CRITICAL"
		}
		alerts = append(alerts, Alert{
			ServiceID: metric.ServiceID,
			AlertName: "HighLatency",
			Severity:  severity,
			Message:   fmt.Sprintf("P95 latency %.0fms exceeded threshold %.0fms", metric.Value, config.LatencyThreshold),
			Timestamp: metric.Timestamp,
		})
	}

	if metric.MetricName == "error_rate" && metric.Value > config.ErrorRateMax {
		severity := "WARNING"
		if metric.Value > config.ErrorRateMax*2 {
			severity = "CRITICAL"
		}
		alerts = append(alerts, Alert{
			ServiceID: metric.ServiceID,
			AlertName: "HighErrorRate",
			Severity:  severity,
			Message:   fmt.Sprintf("Error rate %.1f%% exceeded threshold %.1f%%", metric.Value, config.ErrorRateMax),
			Timestamp: metric.Timestamp,
		})
	}

	if metric.MetricName == "cpu" && metric.Value > 90 {
		alerts = append(alerts, Alert{
			ServiceID: metric.ServiceID,
			AlertName: "HighCPU",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("CPU usage at %.1f%%", metric.Value),
			Timestamp: metric.Timestamp,
		})
	}

	if metric.MetricName == "memory" && metric.Value > 85 {
		alerts = append(alerts, Alert{
			ServiceID: metric.ServiceID,
			AlertName: "HighMemory",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Memory usage at %.1f%%", metric.Value),
			Timestamp: metric.Timestamp,
		})
	}

	// Update SLO gauges
	availability := calculateAvailability(sli)
	budgetRemaining := calculateErrorBudgetRemaining(metric.ServiceID, availability)
	serviceAvailability.WithLabelValues(metric.ServiceID).Set(availability)
	errorBudgetRemaining.WithLabelValues(metric.ServiceID).Set(budgetRemaining)

	// Status is the worst breach across all metrics, so a healthy latency sample
	// can't mask an error rate that is still over threshold
	sli.mu.Lock()
	delete(sli.Breaches, metric.MetricName)
	for _, a := range alerts {
		sli.Breaches[metric.MetricName] = a.Severity
	}
	newStatus := worstStatus(sli.Breaches)
	oldStatus := sli.CurrentStatus
	if newStatus != oldStatus {
		sli.CurrentStatus = newStatus
		sli.mu.Unlock()

		statusChange := ServiceStatusChange{
			ServiceID: metric.ServiceID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			Timestamp: time.Now().Unix(),
			Reason:    fmt.Sprintf("Detected via %s metric evaluation", metric.MetricName),
		}
		statusBytes, _ := json.Marshal(statusChange)
		statusWriter.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(metric.ServiceID),
			Value: statusBytes,
		})
		log.Printf("STATUS CHANGE: %s %s -> %s", metric.ServiceID, oldStatus, newStatus)
	} else {
		sli.mu.Unlock()
	}

	// Publish alerts
	for _, alert := range alerts {
		log.Printf("ALERT TRIGGERED: [%s] %s for %s — %s", alert.Severity, alert.AlertName, alert.ServiceID, alert.Message)
		alertsTriggered.WithLabelValues(alert.ServiceID, alert.AlertName, alert.Severity).Inc()

		alertBytes, _ := json.Marshal(alert)
		err := alertWriter.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(alert.ServiceID),
			Value: alertBytes,
		})
		if err != nil {
			log.Printf("Failed to push alert to Kafka: %v", err)
		}
	}
}

func worstStatus(breaches map[string]string) string {
	status := "HEALTHY"
	for _, sev := range breaches {
		if sev == "CRITICAL" {
			return "INCIDENT"
		}
		status = "DEGRADED"
	}
	return status
}

// ----- Health endpoint -----

func healthHandler(w http.ResponseWriter, r *http.Request) {
	sliMu.RLock()
	defer sliMu.RUnlock()

	type ServiceHealth struct {
		ServiceID       string  `json:"service_id"`
		Status          string  `json:"status"`
		Availability    float64 `json:"availability"`
		ErrorBudgetLeft float64 `json:"error_budget_remaining"`
		SLOTarget       float64 `json:"slo_target"`
		LatencyMs       float64 `json:"latency_ms"`
		LatencyLimitMs  float64 `json:"latency_threshold_ms"`
	}

	report := []ServiceHealth{}
	for id, sli := range serviceSLIs {
		config, ok := sloConfigs[id]
		if !ok {
			config = SLOConfig{Target: 99.9, LatencyThreshold: 500}
		}
		avail := calculateAvailability(sli)
		budget := calculateErrorBudgetRemaining(id, avail)
		sli.mu.RLock()
		report = append(report, ServiceHealth{
			ServiceID:       id,
			Status:          sli.CurrentStatus,
			Availability:    math.Round(avail*100) / 100,
			ErrorBudgetLeft: math.Round(budget*100) / 100,
			SLOTarget:       config.Target,
			LatencyMs:       sli.LatencyLast,
			LatencyLimitMs:  config.LatencyThreshold,
		})
		sli.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func main() {
	broker := getKafkaBroker()

	// Report every registered service from startup, not only ones that have sent metrics
	for id := range sloConfigs {
		getSLI(id)
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    "service.metric.received",
		GroupID:  "alert-engine-group",
		MaxBytes: 10e6,
	})

	alertWriter := &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        "alert.triggered",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // default 1s batch window blocks every synchronous write
	}
	defer alertWriter.Close()

	statusWriter := &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        "service.status.changed",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // default 1s batch window blocks every synchronous write
	}
	defer statusWriter.Close()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", healthHandler)
		log.Println("Alert Engine HTTP server on :8082 (/metrics, /health)")
		log.Fatal(http.ListenAndServe(":8082", mux))
	}()

	log.Println("Starting Alert Engine with SLO evaluation. Waiting for metrics...")

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Error reading message: %v\n", err)
			continue
		}

		var metric Metric
		if err := json.Unmarshal(m.Value, &metric); err != nil {
			log.Printf("Failed to unmarshal metric: %v", err)
			continue
		}

		evaluateMetric(metric, alertWriter, statusWriter)
	}
}
