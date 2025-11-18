# Dockerfile for GoReleaser
# This uses the pre-built binary from GoReleaser's build step
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 scraper && \
    useradd -u 1000 -g scraper -s /bin/bash -m scraper

# Create application directories
RUN mkdir -p /app /config /downloads && \
    chown -R scraper:scraper /app /config /downloads

WORKDIR /app

# Copy the pre-built binary from goreleaser's build context
# With use: buildx, GoReleaser handles the correct binary for each platform
COPY lemmy-scraper .

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
