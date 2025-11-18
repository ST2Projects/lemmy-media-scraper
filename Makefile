.PHONY: build build-fts5 clean test install

# Default build without FTS5 (search will be disabled)
build:
	go build -o lemmy-scraper ./cmd/scraper

# Build with FTS5 full-text search support (recommended)
build-fts5:
	CGO_ENABLED=1 go build -tags fts5 -o lemmy-scraper ./cmd/scraper

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

# Run the application
run: build-fts5
	./lemmy-scraper

# Build for release (with all features)
release: build-fts5
	@echo "Build complete with FTS5 support"
