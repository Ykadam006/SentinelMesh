package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

var topics = []string{
	"service.metric.received",
	"service.status.changed",
	"alert.triggered",
	"incident.created",
	"incident.resolved",
}

var eventsIndexed = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sentinelmesh_events_indexed_total",
	Help: "Kafka events indexed into Elasticsearch",
}, []string{"topic"})

func init() {
	prometheus.MustRegister(eventsIndexed)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// toDoc maps a Kafka message to its Elasticsearch index and a JSON body stamped with @timestamp and topic.
func toDoc(m kafka.Message) (string, []byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(m.Value, &doc); err != nil {
		return "", nil, err
	}
	doc["@timestamp"] = m.Time.UTC().Format(time.RFC3339Nano)
	doc["topic"] = m.Topic
	body, err := json.Marshal(doc)
	return "sentinelmesh-" + strings.ReplaceAll(m.Topic, ".", "-"), body, err
}

func index(esURL, idx string, body []byte) error {
	resp, err := http.Post(esURL+"/"+idx+"/_doc", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch returned %s", resp.Status)
	}
	return nil
}

func main() {
	broker := getEnv("KAFKA_BROKER", "localhost:29092")
	esURL := getEnv("ELASTICSEARCH_URL", "http://localhost:9200")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		GroupID:     "stream-processor-group",
		GroupTopics: topics,
		MaxBytes:    10e6,
	})
	defer r.Close()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Stream Processor HTTP server on :8084 (/metrics)")
		log.Fatal(http.ListenAndServe(":8084", nil))
	}()

	log.Printf("Stream Processor indexing %v into %s", topics, esURL)
	ctx := context.Background()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			log.Printf("Error fetching message: %v", err)
			continue
		}

		idx, body, err := toDoc(m)
		if err != nil {
			log.Printf("Skipping malformed message on %s: %v", m.Topic, err)
		} else {
			// ponytail: one POST per event, switch to _bulk if throughput matters
			for err = index(esURL, idx, body); err != nil; err = index(esURL, idx, body) {
				log.Printf("Index into %s failed, retrying: %v", idx, err)
				time.Sleep(2 * time.Second)
			}
			eventsIndexed.WithLabelValues(m.Topic).Inc()
		}

		// Commit only after the event is safely in Elasticsearch
		if err := r.CommitMessages(ctx, m); err != nil {
			log.Printf("Commit failed: %v", err)
		}
	}
}
