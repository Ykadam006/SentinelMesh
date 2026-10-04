package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	incidentspb "github.com/ykadam006/sentinelmesh/protos/incidents"
	metricspb "github.com/ykadam006/sentinelmesh/protos/metrics"
)

var (
	alertEngineURL  = getEnv("ALERT_ENGINE_URL", "http://localhost:8082")
	metricsClient   metricspb.MetricsServiceClient
	incidentsClient incidentspb.IncidentServiceClient
)

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func reportMetricHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req metricspb.MetricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Timestamp = time.Now().Unix()

	resp, err := metricsClient.ReportMetric(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getIncidentsHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := r.URL.Query().Get("service_id")
	status := r.URL.Query().Get("status")

	resp, err := incidentsClient.GetIncidents(context.Background(), &incidentspb.GetIncidentsRequest{
		ServiceId: serviceID,
		Status:    status,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Incidents)
}

func updateIncidentStatusHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IncidentID string `json:"incident_id"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := incidentsClient.UpdateIncidentStatus(context.Background(), &incidentspb.UpdateIncidentStatusRequest{
		IncidentId: req.IncidentID,
		Status:     req.Status,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// servicesHandler relays per-service SLO health from the alert engine
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(alertEngineURL + "/health")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "api-gateway",
	})
}

func incidentsRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getIncidentsHandler(w, r)
	case "PUT":
		updateIncidentStatusHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	metricsAPIHost := getEnv("METRICS_API_HOST", "localhost:50051")
	incidentAPIHost := getEnv("INCIDENT_API_HOST", "localhost:50052")

	// Connect to Metrics API
	metricsConn, err := grpc.Dial(metricsAPIHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to metrics API: %v", err)
	}
	defer metricsConn.Close()
	metricsClient = metricspb.NewMetricsServiceClient(metricsConn)

	// Connect to Incident API
	incidentsConn, err := grpc.Dial(incidentAPIHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to incident API: %v", err)
	}
	defer incidentsConn.Close()
	incidentsClient = incidentspb.NewIncidentServiceClient(incidentsConn)

	http.HandleFunc("/api/metrics", enableCORS(reportMetricHandler))
	http.HandleFunc("/api/incidents", enableCORS(incidentsRouter))
	http.HandleFunc("/api/services", enableCORS(servicesHandler))
	http.HandleFunc("/health", enableCORS(healthHandler))
	http.Handle("/metrics", promhttp.Handler())

	log.Println("API Gateway listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start API Gateway: %v", err)
	}
}
