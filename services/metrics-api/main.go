package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"

	pb "github.com/ykadam006/sentinelmesh/protos/metrics"
)

var (
	metricsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sentinelmesh_metrics_received_total",
		Help: "Total metrics received by metrics-api",
	}, []string{"service_id", "metric_name"})
)

func init() {
	prometheus.MustRegister(metricsReceived)
}

type server struct {
	pb.UnimplementedMetricsServiceServer
	kafkaWriter *kafka.Writer
}

func (s *server) ReportMetric(ctx context.Context, req *pb.MetricRequest) (*pb.MetricResponse, error) {
	metricsReceived.WithLabelValues(req.ServiceId, req.MetricName).Inc()

	// Serialize metric to JSON for Kafka
	msgBytes, err := json.Marshal(map[string]interface{}{
		"service_id":  req.ServiceId,
		"metric_name": req.MetricName,
		"value":       req.Value,
		"timestamp":   req.Timestamp,
	})
	if err != nil {
		return &pb.MetricResponse{Success: false, Message: "Failed to serialize metric"}, nil
	}

	err = s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(req.ServiceId),
		Value: msgBytes,
	})

	if err != nil {
		log.Printf("Failed to write to kafka: %v", err)
		return &pb.MetricResponse{Success: false, Message: "Failed to push to Kafka"}, nil
	}

	return &pb.MetricResponse{Success: true, Message: "Metric reported"}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	broker := getEnv("KAFKA_BROKER", "localhost:29092")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	kw := &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        "service.metric.received",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond, // default 1s batch window blocks every synchronous write
	}
	defer kw.Close()

	s := grpc.NewServer()
	pb.RegisterMetricsServiceServer(s, &server{kafkaWriter: kw})

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics API HTTP server on :8081 (/metrics)")
		log.Fatal(http.ListenAndServe(":8081", nil))
	}()

	log.Printf("Metrics API gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
