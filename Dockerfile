# Step 1: Build the Go application
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy the source
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o moonrelay main.go

# Final image
FROM gcr.io/distroless/static-debian12

# Copy the built application
COPY --from=builder /app/moonrelay /app/moonrelay

# Copy FFmpeg and FFprobe
COPY --from=mwader/static-ffmpeg:8.0.1 /ffmpeg /usr/local/bin/ffmpeg
COPY --from=mwader/static-ffmpeg:8.0.1 /ffprobe /usr/local/bin/ffprobe

# Set the entrypoint
ENTRYPOINT ["/app/moonrelay"]
