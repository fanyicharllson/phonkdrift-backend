# Stage 1: Build the Go binary executable
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o track-service ./cmd/track-service/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app

# 🛠️ Install ffmpeg, python3 (needed by yt-dlp), nodejs (JS runtime challenge decryptor), and curl
RUN apk add --no-cache ffmpeg python3 nodejs curl

# Download the latest yt-dlp binary directly into system binary path
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Copy compiled binary from builder stage
COPY --from=builder /app/track-service .
COPY cmd/track-service/.env .env

EXPOSE 50052
CMD ["./track-service"]