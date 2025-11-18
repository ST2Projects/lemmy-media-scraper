# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based Lemmy media scraper that downloads images, videos, and other media from Lemmy instances with intelligent content-based deduplication using SHA-256 hashing. The scraper stores comprehensive metadata in SQLite and organizes downloaded files by community.

## Build and Run Commands

**Build with FTS5 search support (recommended):**
```bash
make build-fts5
# Or manually:
CGO_ENABLED=1 go build -tags fts5 -o lemmy-scraper ./cmd/scraper
```

**Build without FTS5 (search will be disabled):**
```bash
make build
# Or manually:
go build -o lemmy-scraper ./cmd/scraper
```

**Note about FTS5:** Full-text search requires SQLite to be built with FTS5 support. The `fts5` build tag ensures the go-sqlite3 driver is compiled with this feature. Without it, the application will still work but search functionality will be disabled.

**Run with default config:**
```bash
./lemmy-scraper
```

**Run with custom config:**
```bash
./lemmy-scraper -config /path/to/config.yaml
```

**Enable verbose logging:**
```bash
./lemmy-scraper -verbose
```

**View statistics:**
```bash
./lemmy-scraper -stats
```

**Disable web server (override config):**
```bash
./lemmy-scraper -no-web
```

**Run tests:**
```bash
go test ./...
```

**Run tests with verbose output:**
```bash
go test -v ./...
```

**Format code:**
```bash
go fmt ./...
```

**Build the web UI:**
```bash
cd web && npm install && npm run build
```

**Run web UI in development mode:**
```bash
cd web && npm run dev
```

## Architecture

### Package Structure

The codebase follows a clean architecture with clear separation of concerns:

- **cmd/scraper/** - Application entry point that orchestrates the scraping workflow
- **internal/config/** - Configuration loading and validation from YAML files
- **internal/api/** - Lemmy API client with JWT authentication
- **internal/database/** - SQLite database operations and schema management
- **internal/downloader/** - Media downloading and content-based deduplication
- **internal/scraper/** - Core scraping logic with pagination support and automatic comments fetching
- **internal/web/** - Optional HTTP server for web UI, API endpoints, and configuration management
- **internal/thumbnails/** - Thumbnail generation for images and videos
- **internal/tags/** - Tag management system with auto-tagging support
- **internal/recognition/** - Image recognition and classification using Ollama
- **internal/progress/** - Real-time progress tracking with WebSocket support
- **pkg/models/** - Shared data models for Lemmy API responses, database records, and comments
- **web/** - SvelteKit web UI for browsing downloaded media

### Key Architectural Patterns

**Deduplication Strategy:**
The scraper uses content-based deduplication, not URL-based. Files are downloaded to memory first, SHA-256 hashed, then checked against the database before writing to disk. This prevents duplicate downloads even if the same media has different URLs.

**Two-Level Tracking:**
1. **scraped_posts table** - Tracks all processed posts (with or without media) to enable intelligent pagination stopping
2. **scraped_media table** - Tracks individual downloaded media files with full metadata

This dual-tracking enables the `stop_at_seen_posts` and `skip_seen_posts` features to work correctly.

**Comments Scraping:**
The scraper automatically fetches and stores comments for posts that had media downloads:
- Uses the `GetComments()` API method with max_depth of 10 and limit of 500
- Stores comments in the `scraped_comments` table with full metadata (author, score, content, path, timestamps)
- Implements idempotency - skips re-fetching comments for posts already in the database
- Filters out removed/deleted comments automatically
- Comments are displayed in the web UI modal viewer with proper threading

**Pagination Model:**
The scraper supports fetching more than the Lemmy API's 50-post limit per request by implementing pagination. It tracks consecutive seen posts and stops intelligently based on the `seen_posts_threshold` config (default: 5).

**Thumbnail Generation:**
Automatically generates optimized thumbnails for images and videos:
- Uses `disintegration/imaging` library for high-quality image resizing
- Extracts video frames using ffmpeg for video thumbnails
- Configurable dimensions, quality, and output directory
- Thumbnails are stored separately and served via `/thumbnails/{id}` endpoint
- Improves web UI performance by serving smaller preview images

**Tag System:**
Flexible tagging system with manual and automatic tag creation:
- User-defined tags with customizable colors
- Auto-tagging via AI image recognition (when enabled)
- Many-to-many relationship between media and tags
- Tags stored in `media_tags` and `media_tag_assignments` tables
- REST API for tag CRUD operations

**Image Recognition & Auto-Tagging:**
Optional AI-powered image classification using Ollama:
- Integrates with local Ollama instance for privacy
- Supports vision models (e.g., llama3.2-vision)
- Automatically generates tags from detected objects and categories
- Optional NSFW content detection
- Configurable confidence threshold for auto-tagging
- All recognition happens locally - no external API calls

**Full-Text Search:**
Fast search across all media using SQLite FTS5:
- Searches post titles, community names, creator names, and URLs
- Auto-synced with media table via triggers
- Supports complex FTS5 query syntax
- Automatically rebuilds index if out of sync
- API endpoint: `GET /api/search?q=query`

**Real-Time Progress Tracking:**
WebSocket-based progress updates during scraping:
- Tracks posts processed, media downloaded, errors
- Calculates estimated time to completion (ETA)
- Broadcasts updates to all connected clients
- Non-blocking progress updates via channels
- WebSocket endpoint: `WS /ws/progress`

**Run Modes:**
- **once** - Single execution (useful for cron jobs)
- **continuous** - Runs on a timer interval with graceful shutdown on SIGTERM/SIGINT

### Configuration System

Configuration is loaded from YAML with validation and sensible defaults:
- Required fields are validated at startup (instance, username, password, storage paths)
- Optional fields have defaults set via `SetDefaults()` method
- Sort types are normalized to match Lemmy API expectations (e.g., "hot" → "Hot")

### Media Type Detection

Media URLs are identified by:
1. File extensions (.jpg, .mp4, .webm, etc.)
2. Known media hosting services (pictrs, imgur, redd.it)

The scraper prioritizes quality:
1. Main post URL (highest quality)
2. Embedded video URL
3. Thumbnail URL (fallback only)

### Database Schema

**scraped_media table:**
- Unique constraint on `media_hash` prevents duplicate downloads
- Composite unique constraint on `(post_id, media_url)` prevents duplicate records from same post
- Indexes on hash, post_id, community_name, and downloaded_at for query performance

**scraped_posts table:**
- Tracks post_id as primary key
- Records whether post had media and count of downloaded items
- Enables idempotent scraping behavior

**scraped_comments table:**
- Stores comment_id as primary key
- Tracks post_id (foreign key relationship to scraped_posts)
- Stores comment content, author info, score, upvotes/downvotes
- Includes comment_path for threading hierarchy
- Indexes on comment_id, post_id, and creator_id for query performance
- Enables rich comment display in web UI

**media_tags table:**
- Stores tag_id as primary key
- Tag name (unique), color, auto_generated flag
- Tracks when tags were created
- Index on tag name for fast lookups

**media_tag_assignments table:**
- Many-to-many relationship between media and tags
- Composite primary key (media_id, tag_id)
- Cascading deletes when media or tags are removed
- Indexes on both media_id and tag_id for efficient queries

**media_thumbnails table:**
- Stores thumbnail metadata per media item
- Thumbnail path, dimensions, generation timestamp
- One-to-one relationship with scraped_media

**media_metadata table:**
- Extended metadata for media items
- Image dimensions, video duration, format, codec
- AI classifications stored as JSON
- NSFW score for content filtering

**scraper_runs table:**
- Tracks each scraper execution for statistics
- Records start/end times, posts processed, media downloaded, errors
- Enables timeline analysis and performance tracking

**media_search_fts (FTS5 Virtual Table):**
- Full-text search index for fast searching
- Indexes post titles, community names, creator names, URLs
- Auto-updated via SQLite triggers on scraped_media changes
- Supports advanced FTS5 query syntax

### Security Features

The scraper implements multiple security layers to protect against common vulnerabilities:

**SSRF Protection:**
- URL scheme whitelist (http, https only)
- Content-Length header validation
- File size limit of 500 MB with LimitReader to prevent memory exhaustion
- URL sanitization preventing javascript: and data: URL schemes

**XSS Prevention:**
- HTML escaping for all user-generated content (post titles, comments)
- Content Security Policy (CSP) headers
- X-Frame-Options and X-Content-Type-Options headers

**Path Traversal Protection:**
- Comprehensive path sanitization in media file serving
- Symlink resolution with base directory validation
- Multiple layers of validation to prevent directory escape attacks

### Web UI Architecture

The optional web interface consists of two components:

**Backend (Go HTTP Server):**
- Serves RESTful API endpoints for querying the SQLite database
- Serves static media files from the downloads directory
- Serves the compiled SvelteKit frontend
- Runs in a goroutine alongside the scraper
- API endpoints:
  - `GET /api/media` - Paginated media list with filtering (community, type, sort)
  - `GET /api/media/:id` - Individual media item details
  - `GET /api/comments/:mediaId` - Get comments for a specific media item
  - `GET /api/stats` - Overall statistics
  - `GET /api/communities` - List of communities with media counts
  - `GET /api/config` - Get current configuration (web-based config management)
  - `PUT /api/config` - Update configuration (web-based config management)
  - `GET /api/search?q=query` - Full-text search across media
  - `GET /api/tags` - List all tags
  - `POST /api/tags` - Create a new tag
  - `GET /api/tags/:id` - Get tag by ID
  - `DELETE /api/tags/:id` - Delete a tag
  - `GET /api/media-tags/:mediaId` - Get tags for a media item
  - `POST /api/media-tags/:mediaId` - Assign tag to media
  - `DELETE /api/media-tags/:mediaId/:tagId` - Remove tag from media
  - `GET /api/stats/timeline?period=day|week|month` - Download statistics over time
  - `GET /api/stats/top-creators?limit=10` - Top content creators
  - `GET /api/stats/storage` - Storage breakdown by community and type
  - `WS /ws/progress` - WebSocket for real-time progress updates
  - `GET /media/{community}/{filename}` - Serve actual media files
  - `GET /thumbnails/:mediaId` - Serve thumbnail for media item
  - `GET /settings` - Web-based configuration management UI

**Frontend (SvelteKit + Skeleton UI):**
- Modern Svelte 5 with runes syntax (`$state`, `$derived`)
- Skeleton UI component library with Tailwind CSS
- TypeScript for type safety
- Features:
  - Responsive grid layout for media thumbnails
  - Filtering by community and media type
  - Sorting by download date, post date, file size, or score
  - Pagination for large media libraries
  - Modal viewer for full-size images and videos with threaded comments display
  - Statistics dashboard showing totals and breakdowns
  - Web-based configuration management via `/settings` page

**Integration:**
- Web server is completely optional (enabled by default in code, can be disabled in config or via `-no-web` flag)
- When enabled in "once" mode, the scraper runs first, then the web server stays up
- In "continuous" mode, the web server runs concurrently with scheduled scrapes
- CORS is enabled to allow development mode (SvelteKit dev server) to access the API
- Production builds are served as static files from `web/build/`
- The `/settings` page allows runtime configuration changes via web UI

## Development Guidelines

### Adding New Features

When modifying the scraper behavior:
- Update both the `ScraperConfig` struct in `internal/config/config.go` and the example YAML
- Add validation in the `Validate()` method if the field is required
- Add defaults in the `SetDefaults()` method if the field is optional

### Working with the API Client

The Lemmy API client (`internal/api/client.go`) uses JWT authentication:
- Login once at startup, store the JWT token
- Include `Authorization: Bearer <token>` header in all subsequent requests
- API uses v3 endpoints (`/api/v3/...`)

**Comments API:**
- `GetComments()` method fetches comments for a specific post
- Parameters: max_depth (default: 10), limit (default: 500), sort (default: "Top")
- Returns threaded comment structure with full metadata
- Automatically called after successful media downloads

### Database Operations

When adding new database queries:
- Use prepared statements (the `?` placeholder syntax)
- Handle `sql.ErrNoRows` separately from other errors
- Add appropriate indexes for new query patterns
- Remember to update the schema version if changing table structure

### Error Handling Philosophy

The scraper is designed to be fault-tolerant:
- Individual post failures don't stop the entire scrape
- Media download errors are logged but don't crash the application
- In continuous mode, errors in one run don't prevent subsequent runs

### Logging

Uses logrus with two levels:
- **Info** (default) - High-level progress and summary statistics
- **Debug** (`-verbose` flag) - Detailed operation logs including API requests and individual post processing

## File Organization

Downloaded media is organized as:
```
{base_directory}/
  {community_name}/
    {post_id}_{original_filename}
```

This structure allows easy browsing by community while preserving post IDs for cross-referencing with the database.

## Configuration Format

Key configuration sections:

**lemmy.communities:**
- Empty list `[]` scrapes from instance hot page
- Can specify communities as simple names `["technology"]` or fully qualified `["technology@lemmy.ml"]`

**scraper.max_posts_per_run:**
- Total posts across all pages
- If pagination disabled, automatically capped at 50 (API max)

**scraper.stop_at_seen_posts vs skip_seen_posts:**
- `stop_at_seen_posts: true` - Stop scraping after hitting threshold of consecutive seen posts
- `skip_seen_posts: true` - Skip seen posts but continue scraping (use with caution on large communities)

**scraper.seen_posts_threshold:**
- Number of consecutive seen posts before stopping (when `stop_at_seen_posts: true`)
- Default: 5
- Prevents premature stopping while ensuring efficiency

**web_server.enabled:**
- Set to `true` to enable the web UI for browsing downloaded media
- Default: `true` (enabled by default in code)
- Can be disabled in config file or via `-no-web` command-line flag
- When enabled, starts an HTTP server alongside the scraper with web UI and configuration management

**web_server.host:**
- Host/interface to bind the web server to
- Default: `localhost` (only accessible from local machine)
- Use `0.0.0.0` to allow external network access

**web_server.port:**
- TCP port for the web server
- Default: `8080`
- Access the web UI at `http://{host}:{port}`

**thumbnails.enabled:**
- Enable automatic thumbnail generation (default: `true`)
- Generates optimized preview images for faster web UI loading
- Requires `disintegration/imaging` library

**thumbnails.max_width and thumbnails.max_height:**
- Maximum dimensions for thumbnails (default: 400x400)
- Aspect ratio is preserved during resizing

**thumbnails.quality:**
- JPEG quality for thumbnails (1-100, default: 85)
- Higher values = better quality but larger file sizes

**thumbnails.directory:**
- Directory to store thumbnail files (default: `./thumbnails`)
- Separate from main media storage

**thumbnails.video_method:**
- Method for video thumbnail generation (default: `ffmpeg`)
- Requires ffmpeg to be installed and in PATH

**recognition.enabled:**
- Enable AI-powered image recognition (default: `false`)
- Requires Ollama with a vision model installed locally

**recognition.provider:**
- Recognition provider (default: `ollama`)
- Currently only Ollama is supported

**recognition.ollama_url:**
- Ollama API URL (default: `http://localhost:11434`)
- Must be a local or accessible Ollama instance

**recognition.model:**
- Model to use for classification (default: `llama3.2-vision:latest`)
- Install with: `ollama pull llama3.2-vision:latest`

**recognition.auto_tag:**
- Automatically create and assign tags from classifications (default: `true`)
- Tags are created with `auto_generated` flag

**recognition.nsfw_detection:**
- Enable NSFW content detection (default: `false`)
- Experimental feature, scores stored in media_metadata table

**recognition.confidence_threshold:**
- Minimum confidence for auto-tagging (0.0-1.0, default: 0.6)
- Higher values = more selective tagging

**search.rebuild_index:**
- Rebuild the FTS5 search index on startup (default: `false`)
- Only needed if search results seem incorrect or after schema changes

## Web UI Development

When working on the web interface:

**Development workflow:**
1. Start the Go backend with web server enabled
2. In a separate terminal, run `cd web && npm run dev` for hot-reload frontend development
3. The dev server (usually port 5173) will proxy API calls to the Go backend (port 8080)

**Building for production:**
1. Run `cd web && npm run build` to create optimized static files
2. The Go server will automatically serve these from `web/build/`
3. If build doesn't exist, Go server shows a helpful message with build instructions

**Modifying the API:**
- Add new endpoints in `internal/web/server.go`
- Update TypeScript interfaces in the SvelteKit pages as needed
- Remember to handle CORS for development mode

**Web-Based Configuration Management:**
- The `/settings` page provides a web UI for editing configuration
- `GET /api/config` returns current configuration as JSON
- `PUT /api/config` updates configuration (saved to config file)
- Configuration changes require application restart to take effect
- All config fields are editable via form UI with validation

**Styling with Skeleton:**
- Use Skeleton's semantic class names (e.g., `btn`, `card`, `badge`)
- Theme colors use `variant-*` classes (e.g., `variant-filled-primary`)
- Surface colors auto-adapt to light/dark mode with `surface-*-token` classes
