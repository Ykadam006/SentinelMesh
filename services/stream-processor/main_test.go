package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestToDoc(t *testing.T) {
	ts := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	idx, body, err := toDoc(kafka.Message{Topic: "alert.triggered", Time: ts, Value: []byte(`{"service_id":"payment-service"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if idx != "sentinelmesh-alert-triggered" {
		t.Errorf("index = %q", idx)
	}
	var doc map[string]any
	json.Unmarshal(body, &doc)
	if doc["service_id"] != "payment-service" || doc["topic"] != "alert.triggered" || doc["@timestamp"] != "2026-10-03T12:00:00Z" {
		t.Errorf("doc = %v", doc)
	}
	if _, _, err := toDoc(kafka.Message{Value: []byte("not json")}); err == nil {
		t.Error("expected error on malformed message")
	}
}
