package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pb "github.com/ykadam006/sentinelmesh/protos/incidents"
)

// ----- DB Models -----

type Incident struct {
	ID          string `gorm:"primaryKey" json:"id"`
	ServiceID   string `json:"service_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // P1, P2, P3, P4
	Status      string `json:"status"`   // ACTIVE, ACKNOWLEDGED, RESOLVED
	CreatedAt   int64  `json:"created_at"`
	AckedAt     int64  `json:"acked_at"`
	ResolvedAt  int64  `json:"resolved_at"`
	MTTR        int64  `json:"mttr"` // Mean Time To Resolution in seconds
}

// IncidentTimelineEntry tracks the lifecycle of an incident
type IncidentTimelineEntry struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	IncidentID string `json:"incident_id"`
	Event      string `json:"event"` // CREATED, ACKNOWLEDGED, ESCALATED, RESOLVED
	Details    string `json:"details"`
	Timestamp  int64  `json:"timestamp"`
}

// Postmortem is auto-generated when an incident is resolved
type Postmortem struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	IncidentID   string `gorm:"uniqueIndex" json:"incident_id"`
	ServiceID    string `json:"service_id"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	RootCause    string `json:"root_cause"`
	Impact       string `json:"impact"`
	MTTRSeconds  int64  `json:"mttr_seconds"`
	CreatedAt    int64  `json:"created_at"`
	TimelineJSON string `json:"timeline_json"` // JSON-encoded timeline
}

type Alert struct {
	ServiceID string `json:"service_id"`
	AlertName string `json:"alert_name"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// ----- Prometheus Metrics -----

var (
	incidentsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sentinelmesh_incidents_created_total",
		Help: "Total incidents created",
	}, []string{"service_id", "severity"})

	incidentsResolved = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sentinelmesh_incidents_resolved_total",
		Help: "Total incidents resolved",
	}, []string{"service_id"})

	mttrHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "sentinelmesh_mttr_seconds",
		Help:    "Mean Time To Resolution distribution",
		Buckets: []float64{60, 300, 600, 1800, 3600, 7200, 14400},
	}, []string{"service_id"})
)

func init() {
	prometheus.MustRegister(incidentsCreated, incidentsResolved, mttrHistogram)
}

// ----- gRPC Server -----

type server struct {
	pb.UnimplementedIncidentServiceServer
	db         *gorm.DB
	kwResolved *kafka.Writer
	kwCreated  *kafka.Writer
}

func (s *server) GetIncidents(ctx context.Context, req *pb.GetIncidentsRequest) (*pb.GetIncidentsResponse, error) {
	var incidents []Incident
	query := s.db.Model(&Incident{}).Order("created_at DESC")

	if req.ServiceId != "" {
		query = query.Where("service_id = ?", req.ServiceId)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	query.Find(&incidents)

	var pbIncidents []*pb.Incident
	for _, inc := range incidents {
		pbIncidents = append(pbIncidents, &pb.Incident{
			Id:          inc.ID,
			ServiceId:   inc.ServiceID,
			Title:       inc.Title,
			Description: inc.Description,
			Status:      inc.Status,
			CreatedAt:   inc.CreatedAt,
			ResolvedAt:  inc.ResolvedAt,
		})
	}
	return &pb.GetIncidentsResponse{Incidents: pbIncidents}, nil
}

func (s *server) UpdateIncidentStatus(ctx context.Context, req *pb.UpdateIncidentStatusRequest) (*pb.UpdateIncidentStatusResponse, error) {
	var incident Incident
	if err := s.db.First(&incident, "id = ?", req.IncidentId).Error; err != nil {
		return &pb.UpdateIncidentStatusResponse{Success: false}, nil
	}

	oldStatus := incident.Status
	incident.Status = req.Status
	now := time.Now().Unix()

	if req.Status == "ACKNOWLEDGED" && incident.AckedAt == 0 {
		incident.AckedAt = now
	}

	if req.Status == "RESOLVED" {
		incident.ResolvedAt = now
		incident.MTTR = now - incident.CreatedAt
		incidentsResolved.WithLabelValues(incident.ServiceID).Inc()
		mttrHistogram.WithLabelValues(incident.ServiceID).Observe(float64(incident.MTTR))
	}

	s.db.Save(&incident)

	// Record timeline entry
	s.db.Create(&IncidentTimelineEntry{
		IncidentID: incident.ID,
		Event:      req.Status,
		Details:    fmt.Sprintf("Status changed from %s to %s", oldStatus, req.Status),
		Timestamp:  now,
	})

	if req.Status == "RESOLVED" {
		go s.generatePostmortem(incident)

		incidentBytes, _ := json.Marshal(incident)
		s.kwResolved.WriteMessages(ctx, kafka.Message{
			Key:   []byte(incident.ServiceID),
			Value: incidentBytes,
		})
	}

	return &pb.UpdateIncidentStatusResponse{
		Success: true,
		Incident: &pb.Incident{
			Id:          incident.ID,
			ServiceId:   incident.ServiceID,
			Title:       incident.Title,
			Description: incident.Description,
			Status:      incident.Status,
			CreatedAt:   incident.CreatedAt,
			ResolvedAt:  incident.ResolvedAt,
		},
	}, nil
}

func (s *server) generatePostmortem(incident Incident) {
	var timeline []IncidentTimelineEntry
	s.db.Where("incident_id = ?", incident.ID).Order("timestamp ASC").Find(&timeline)

	timelineJSON, _ := json.Marshal(timeline)

	postmortem := Postmortem{
		IncidentID:   incident.ID,
		ServiceID:    incident.ServiceID,
		Title:        fmt.Sprintf("Postmortem: %s — %s", incident.Title, incident.ServiceID),
		Summary:      fmt.Sprintf("Incident %s was detected and resolved. MTTR: %d seconds.", incident.ID, incident.MTTR),
		RootCause:    incident.Description,
		Impact:       fmt.Sprintf("Service %s was in %s status for %d seconds.", incident.ServiceID, "INCIDENT", incident.MTTR),
		MTTRSeconds:  incident.MTTR,
		CreatedAt:    time.Now().Unix(),
		TimelineJSON: string(timelineJSON),
	}

	s.db.Create(&postmortem)
	log.Printf("POSTMORTEM generated for incident %s (MTTR: %ds)", incident.ID, incident.MTTR)
}

// ----- Kafka Consumer -----

func (s *server) consumeAlerts() {
	broker := getKafkaBroker()
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    "alert.triggered",
		GroupID:  "incident-api-group",
		MaxBytes: 10e6,
	})

	log.Println("Incident API listening for alerts...")
	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Error reading alert: %v", err)
			continue
		}

		var alert Alert
		if err := json.Unmarshal(m.Value, &alert); err != nil {
			log.Printf("Failed to unmarshal alert: %v", err)
			continue
		}

		log.Printf("Received alert [%s] for %s. Creating incident...", alert.Severity, alert.ServiceID)

		severity := "P3"
		switch alert.Severity {
		case "CRITICAL":
			severity = "P1"
		case "WARNING":
			severity = "P2"
		}

		now := time.Now().Unix()

		// Dedup: repeated alerts attach to the open incident instead of opening a new one
		var open Incident
		if s.db.Where("service_id = ? AND title = ? AND status <> ?", alert.ServiceID, alert.AlertName, "RESOLVED").
			Order("created_at DESC").First(&open).Error == nil {
			s.db.Create(&IncidentTimelineEntry{
				IncidentID: open.ID,
				Event:      "ALERT_REPEATED",
				Details:    fmt.Sprintf("[%s] %s", alert.Severity, alert.Message),
				Timestamp:  now,
			})
			continue
		}

		incidentID := fmt.Sprintf("INC-%s-%s-%s", alert.ServiceID, alert.AlertName, time.Now().Format("20060102150405"))

		incident := Incident{
			ID:          incidentID,
			ServiceID:   alert.ServiceID,
			Title:       alert.AlertName,
			Description: alert.Message,
			Severity:    severity,
			Status:      "ACTIVE",
			CreatedAt:   now,
		}
		if err := s.db.Create(&incident).Error; err != nil {
			log.Printf("Failed to create incident %s: %v", incidentID, err)
			continue
		}
		incidentsCreated.WithLabelValues(alert.ServiceID, severity).Inc()

		// Create timeline entry
		s.db.Create(&IncidentTimelineEntry{
			IncidentID: incidentID,
			Event:      "CREATED",
			Details:    fmt.Sprintf("Alert %s triggered: %s", alert.AlertName, alert.Message),
			Timestamp:  now,
		})

		incidentBytes, _ := json.Marshal(incident)
		s.kwCreated.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(incident.ServiceID),
			Value: incidentBytes,
		})
	}
}

// ----- Helpers -----

func getKafkaBroker() string {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:29092"
	}
	return broker
}

func getDSN() string {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "sentinel"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "password"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "sentinelmesh"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)
}

func main() {
	broker := getKafkaBroker()

	kwCreated := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    "incident.created",
		Balancer: &kafka.LeastBytes{},
	}
	defer kwCreated.Close()

	kwResolved := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    "incident.resolved",
		Balancer: &kafka.LeastBytes{},
	}
	defer kwResolved.Close()

	dsn := getDSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Failed to connect database, will retry later... %v", err)
	} else {
		db.AutoMigrate(&Incident{}, &IncidentTimelineEntry{}, &Postmortem{})
	}

	srv := &server{db: db, kwResolved: kwResolved, kwCreated: kwCreated}

	if db != nil {
		go srv.consumeAlerts()
	}

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterIncidentServiceServer(s, srv)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Incident API HTTP server on :8083 (/metrics)")
		log.Fatal(http.ListenAndServe(":8083", nil))
	}()

	log.Printf("Incident API gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
