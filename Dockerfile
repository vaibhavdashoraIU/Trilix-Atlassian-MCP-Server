# Multi-service Dockerfile for Trilix Atlassian MCP Server
# Build with: docker build --build-arg SERVICE=<service-name> -t <image-name> .
# Example: docker build --build-arg SERVICE=mcp-server -t trilix/mcp-server .

# Build stage
FROM golang:alpine AS builder

ARG SERVICE
WORKDIR /app

# Copy source code and vendor directory
COPY . .

# Build the binary based on SERVICE arg (using vendor)
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo \
    -o service-binary ./cmd/${SERVICE}/main.go

# Runtime stage
FROM alpine:latest

ARG SERVICE
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/service-binary ./service

# Copy config files for this service
COPY --from=builder /app/cmd/${SERVICE}/config.yaml ./config.yaml
COPY --from=builder /app/cmd/${SERVICE}/settings.yaml ./settings.yaml

# Copy frontend files only for mcp-server
COPY --from=builder /app/frontend ./frontend

# Set environment variables for containerized paths
ENV FRONTEND_PATH=/root/frontend

# Copy entrypoint script
COPY --from=builder /app/entrypoint.sh .
RUN chmod +x entrypoint.sh

# Expose ports (3000 for mcp-server, 8080 for others)
EXPOSE 3000 8080

CMD ["./entrypoint.sh"]
