# Stage 1: Build the Go binary executable
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway ./cmd/api-gateway/main.go

# Stage 2: Final lightweight runtime environment
FROM alpine:3.19
WORKDIR /app

# yt-dlp needed for manual track seeding from admin dashboard
RUN apk add --no-cache \
    python3 \
    py3-pip \
    ffmpeg \
    curl \
    ca-certificates

RUN pip3 install --break-system-packages yt-dlp

# Tell yt-dlp to use nodejs as JS runtime
ENV YT_DLP_JS_RUNTIME=nodejs


COPY --from=builder /app/api-gateway .

# Expose ports for both mobile gRPC traffic and HTTP Postman testing
EXPOSE 50050 8080
CMD ["./api-gateway"]