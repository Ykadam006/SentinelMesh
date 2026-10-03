FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/go.mod protos/go.sum ./protos/
COPY services/incident-api/go.mod services/incident-api/go.sum ./services/incident-api/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/incident-api
RUN go mod download
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY protos/ ./protos/
COPY services/incident-api/ ./services/incident-api/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/incident-api
RUN CGO_ENABLED=0 go build -o /incident-api .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /incident-api /incident-api
EXPOSE 50052 8083
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8083/metrics || exit 1
ENTRYPOINT ["/incident-api"]
