# Dockerfile for Knowledge Engine Backend
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy dependency definitions
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o knowledge-engine ./cmd/server/main.go

# Final Stage (Minimal Image)
FROM alpine:latest

WORKDIR /app

# Add certificates for HTTPS crawling
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/knowledge-engine .

# Create data directory for persistence
RUN mkdir -p data

# Expose API port
EXPOSE 8080

# Run the service
CMD ["./knowledge-engine"]
