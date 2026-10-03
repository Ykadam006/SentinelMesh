FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/go.mod protos/go.sum ./protos/
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./services/api-gateway/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/api-gateway
RUN go mod download
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/ ./protos/
COPY services/api-gateway/ ./services/api-gateway/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/api-gateway
RUN CGO_ENABLED=0 go build -o /api-gateway .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /api-gateway /api-gateway
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8080/metrics || exit 1
ENTRYPOINT ["/api-gateway"]
