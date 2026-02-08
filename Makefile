.PHONY: build build-fts5 clean test install web all dev-web docker-build docker-build-web docker-up docker-down

# Default build without FTS5 (search will be disabled)
build:
	go build -o lemmy-scraper ./cmd/scraper

# Build with FTS5 full-text search support (recommended)
build-fts5:
	CGO_ENABLED=1 go build -tags fts5 -o lemmy-scraper ./cmd/scraper

# Build SvelteKit frontend
web:
	cd web && npm ci && npm run build

# Build everything (local development)
all: web build-fts5

# Install dependencies
install:
	go mod download
	go mod tidy

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f lemmy-scraper
	rm -rf dist/
	rm -rf web/build/

# Run the application
run: build-fts5
	./lemmy-scraper

# Run web UI in development mode
dev-web:
	cd web && npm run dev

# Build Docker image for the Go backend
docker-build:
	CGO_ENABLED=1 go build -tags fts5 -o lemmy-scraper ./cmd/scraper
	docker build -t lemmy-media-scraper:latest .

# Build Docker image for the SvelteKit web UI
docker-build-web:
	docker build -f Dockerfile.web -t lemmy-media-scraper-web:latest .

# Build both Docker images
docker-build-all: docker-build docker-build-web

# Start all services with docker compose
docker-up:
	docker compose up -d

# Stop all services
docker-down:
	docker compose down

# Build for release (with all features)
release: build-fts5
	@echo "Build complete with FTS5 support"
