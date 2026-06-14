# Stage 1: Build the Go binary executable (Updated to Go 1.25)
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy dependency manifests and download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire monorepo source code
COPY . .

# Compile the static Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -o track-service ./cmd/track-service/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app

# Install runtime dependencies for the audio extractor engine
RUN apk add --no-cache ffmpeg python3 nodejs curl

# Download and authorize the latest version of yt-dlp
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Copy compiled executable binary from build manager stage
COPY --from=builder /app/track-service .

EXPOSE 50052
CMD ["./track-service"]