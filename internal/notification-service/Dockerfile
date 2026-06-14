# Stage 1: Build the Go binary executable
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o notification-service ./cmd/notification-service/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/notification-service .

# Notification service usually doesn't expose gRPC ports since it's an async consumer, 
# but we expose a health check port if you implemented one (e.g., 50053)
EXPOSE 50053
CMD ["./notification-service"]