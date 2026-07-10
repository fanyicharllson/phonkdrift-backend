# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o discovery-service ./cmd/discovery-service/main.go

# Stage 2: Runtime with yt-dlp (node:22-alpine ships a musl-compatible Node 22 build,
# which is yt-dlp's minimum supported version for JS challenge solving)
FROM node:22-alpine
WORKDIR /app

# Install yt-dlp + ffmpeg (needed for audio extraction/conversion)
RUN apk add --no-cache \
    python3 \
    py3-pip \
    ffmpeg \
    curl \
    ca-certificates

# yt-dlp[default] pulls in yt-dlp-ejs, the JS challenge-solver scripts required
# for YouTube signature/n-parameter deciphering
RUN pip3 install --break-system-packages "yt-dlp[default]"

# Only Deno is enabled by default for JS challenge solving; Node must be enabled explicitly
RUN echo '--js-runtimes node' > /etc/yt-dlp.conf

COPY --from=builder /app/discovery-service .

CMD ["./discovery-service"]