# Dockerfile for the Go backend (lemmy-scraper)
# This Dockerfile expects the Go binary to be pre-built (e.g., by GoReleaser)
# and available in the build context as 'lemmy-scraper'.
#
# For local development, build the binary first:
#   CGO_ENABLED=1 go build -tags fts5 -o lemmy-scraper ./cmd/scraper

FROM debian:bookworm-slim

# Install runtime dependencies (ffmpeg for video thumbnails, ca-certificates for HTTPS)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 scraper && \
    useradd -u 1000 -g scraper -s /bin/bash -m scraper

# Create application directories
RUN mkdir -p /app /config /downloads /thumbnails && \
    chown -R scraper:scraper /app /config /downloads /thumbnails

WORKDIR /app

# Copy pre-built Go binary
COPY --chown=scraper:scraper lemmy-scraper .

# Switch to non-root user
USER scraper

# Define volumes for persistent data
VOLUME ["/config", "/downloads", "/thumbnails"]

# Expose Go API port
EXPOSE 8081

# Set environment variables with defaults
ENV CONFIG_PATH=/config/config.yaml

# Health check against the Go API server
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD pgrep -x lemmy-scraper || exit 1

ENTRYPOINT ["/app/lemmy-scraper"]
CMD ["-config", "/config/config.yaml"]
