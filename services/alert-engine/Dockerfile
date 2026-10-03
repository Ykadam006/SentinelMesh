FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY services/alert-engine/go.mod services/alert-engine/go.sum ./services/alert-engine/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/alert-engine
RUN go mod download
ENV GOTOOLCHAIN=auto

WORKDIR /app
COPY services/alert-engine/ ./services/alert-engine/
ENV GOTOOLCHAIN=auto

WORKDIR /app/services/alert-engine
RUN CGO_ENABLED=0 go build -o /alert-engine .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /alert-engine /alert-engine
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8082/metrics || exit 1
ENTRYPOINT ["/alert-engine"]
