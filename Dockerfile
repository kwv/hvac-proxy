# Build stage
FROM golang:1.25-alpine AS builder

# Set labels *after* the FROM instruction
ARG VERSION=local
LABEL maintainer="kwv4"
LABEL version="$VERSION"
LABEL description="A lightweight HTTP proxy for Carrier/Bryant Infinity HVAC systems that logs XML traffic and exposes Prometheus-compatible metrics."
LABEL repository="https://github.com/kwv/hvac-proxy"

WORKDIR /app

# Copy go.mod and go.sum first (for better caching)
COPY go.mod ./
COPY go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# Use a common Alpine base for better compatibility with distroless later
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hvac-proxy .

# Final stage - distroless
# Changed base image to a secure, stable image from the Google project
FROM gcr.io/distroless/static-debian12:nonroot

# Copy over the trusted certificates for HTTPS to work (critical for distroless)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/hvac-proxy /app/hvac-proxy

# Expose proxy port
EXPOSE 8080

# Run as non-root user
USER nonroot:nonroot

# Run the proxy
ENTRYPOINT ["/app/hvac-proxy"]