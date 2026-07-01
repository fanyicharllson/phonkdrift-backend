# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o discovery-service ./cmd/discovery-service/main.go

# Stage 2: Runtime with yt-dlp
FROM alpine:3.19
WORKDIR /app

# Install yt-dlp + ffmpeg (needed for audio extraction/conversion)
RUN apk add --no-cache \
    python3 \
    py3-pip \
    ffmpeg \
    curl \
    ca-certificates \
    nodejs \          
    npm  

RUN pip3 install --break-system-packages yt-dlp

COPY --from=builder /app/discovery-service .

CMD ["./discovery-service"]