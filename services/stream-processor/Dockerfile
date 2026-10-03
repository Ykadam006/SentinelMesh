FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY services/stream-processor/go.mod services/stream-processor/go.sum ./services/stream-processor/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/stream-processor
RUN go mod download
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY services/stream-processor/ ./services/stream-processor/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/stream-processor
RUN CGO_ENABLED=0 go build -o /stream-processor .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /stream-processor /stream-processor
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8084/metrics || exit 1
ENTRYPOINT ["/stream-processor"]
