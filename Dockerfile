# Dockerfile for GoReleaser
# This uses the pre-built binary from GoReleaser's build step
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 scraper && \
    adduser -D -u 1000 -G scraper scraper

# Create application directories
RUN mkdir -p /app /config /downloads && \
    chown -R scraper:scraper /app /config /downloads

WORKDIR /app

# Copy the pre-built binary from goreleaser's build context
# GoReleaser provides TARGETPLATFORM (e.g., linux/amd64, linux/arm64)
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/lemmy-scraper .

# Switch to non-root user
USER scraper

# Define volumes for persistent data
VOLUME ["/config", "/downloads"]

# Expose web UI port
EXPOSE 8080

# Set environment variables with defaults
ENV CONFIG_PATH=/config/config.yaml

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD pgrep -x lemmy-scraper || exit 1

# Run the application
ENTRYPOINT ["/app/lemmy-scraper"]
CMD ["-config", "/config/config.yaml"]
