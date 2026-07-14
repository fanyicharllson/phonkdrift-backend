# Stage 1: Build the Go binary executable
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o chat-service ./cmd/chat-service/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/chat-service .

EXPOSE 50053
CMD ["./chat-service"]