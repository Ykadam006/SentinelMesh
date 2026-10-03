#!/bin/bash

echo "🚀 Starting Chaos Engineering Scenario: Payment Service Latency Spike"

# Target the API Gateway /metrics endpoint to simulate bad metrics
API_URL="http://localhost:8080/api/metrics"

echo "Normal traffic baseline..."
for i in {1..5}; do
  curl -s -X POST $API_URL -d '{"service_id": "payment-service", "metric_name": "latency", "value": 150}' -H "Content-Type: application/json" > /dev/null
  sleep 1
done

echo "🔥 Injecting artificial latency into payment-service..."
for i in {1..10}; do
  # Simulating latency over 500ms which triggers the alert
  LATENCY=$((500 + RANDOM % 500)) 
  echo "Sending latency metric: ${LATENCY}ms"
  curl -s -X POST $API_URL -d "{\"service_id\": \"payment-service\", \"metric_name\": \"latency\", \"value\": $LATENCY}" -H "Content-Type: application/json" > /dev/null
  sleep 0.5
done

echo "Traffic returning to normal..."
for i in {1..5}; do
  curl -s -X POST $API_URL -d '{"service_id": "payment-service", "metric_name": "latency", "value": 180}' -H "Content-Type: application/json" > /dev/null
  sleep 1
done

echo "✅ Chaos scenario completed."
