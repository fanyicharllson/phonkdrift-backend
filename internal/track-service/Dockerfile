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

# Tell yt-dlp to use nodejs as JS runtime
ENV YT_DLP_JS_RUNTIME=nodejs

# Copy compiled executable binary from build manager stage
COPY --from=builder /app/track-service .

EXPOSE 50052
CMD ["./track-service"]