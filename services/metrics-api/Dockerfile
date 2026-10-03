FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/go.mod protos/go.sum ./protos/
COPY services/metrics-api/go.mod services/metrics-api/go.sum ./services/metrics-api/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/metrics-api
RUN go mod download
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/ ./protos/
COPY services/metrics-api/ ./services/metrics-api/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/metrics-api
RUN CGO_ENABLED=0 go build -o /metrics-api .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /metrics-api /metrics-api
EXPOSE 50051 8081
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8081/metrics || exit 1
ENTRYPOINT ["/metrics-api"]
