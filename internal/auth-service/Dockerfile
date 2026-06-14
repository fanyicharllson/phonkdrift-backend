# Stage 1: Build the Go binary executable
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service ./cmd/auth-service/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/auth-service .

EXPOSE 50051
CMD ["./auth-service"]