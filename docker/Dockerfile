# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN make build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -g '' gojira

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/dist/gojira /app/gojira

# Copy default config
COPY --from=builder /build/configs /app/configs

# Change ownership
RUN chown -R gojira:gojira /app

# Use non-root user
USER gojira

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/gojira", "health"] || exit 1

ENTRYPOINT ["/app/gojira"]
CMD ["serve"]