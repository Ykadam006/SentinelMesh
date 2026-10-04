#!/bin/bash
# Chaos scenario: payment-service degrades (latency spike + failing requests), then recovers.
# Watch it on the dashboard (http://localhost), Grafana (:3000) and Kibana (:5601).

API_URL="${API_URL:-http://localhost:8080/api/metrics}"
SERVICE="${SERVICE:-payment-service}"

send() {
  curl -s -X POST "$API_URL" -H "Content-Type: application/json" \
    -d "{\"service_id\": \"$SERVICE\", \"metric_name\": \"$1\", \"value\": $2}" > /dev/null
}

echo "🚀 Chaos scenario: $SERVICE outage"

echo "Normal traffic baseline..."
for i in {1..5}; do
  send request_rate 30000
  send latency $((150 + RANDOM % 40))
  send error_rate 0
  sleep 1
done

echo "🔥 Injecting latency and failures into $SERVICE..."
for i in {1..10}; do
  LATENCY=$((900 + RANDOM % 900))   # well over the 500ms threshold -> HighLatency
  ERRORS=$((8 + RANDOM % 7))         # 8-14% errors -> HighErrorRate, burns error budget
  echo "  latency ${LATENCY}ms, error rate ${ERRORS}%"
  send latency "$LATENCY"
  send error_rate "$ERRORS"
  sleep 0.5
done

echo "Traffic returning to normal..."
for i in {1..5}; do
  send latency $((160 + RANDOM % 40))
  send error_rate 0
  sleep 1
done

echo "✅ Chaos scenario completed. Resolve the incident on the dashboard to generate a postmortem."
