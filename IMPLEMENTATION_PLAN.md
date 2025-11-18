# Implementation Plan: Advanced Features

This document outlines the implementation plan for adding 6 major features to lemmy-image-scraper.

## Features Overview

1. **Full-Text Search (FTS5)** - Search across posts, comments, and communities
2. **Tag/Label System** - User-defined tags with auto-tagging
3. **Advanced Statistics Dashboard** - Timeline charts, analytics, breakdowns
4. **Thumbnail Generation** - Efficient thumbnails for images and videos
5. **Image Recognition & Auto-tagging** - AI-powered classification using local models
6. **Real-time Progress** - WebSocket updates during scraping

## Architecture Changes

### Database Schema Changes

#### New Tables

**1. `media_tags` - Tag definitions**
```sql
CREATE TABLE media_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT,
    auto_generated BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tags_name ON media_tags(name);
```

**2. `media_tag_assignments` - Many-to-many relationship**
```sql
CREATE TABLE media_tag_assignments (
    media_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (media_id, tag_id),
    FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES media_tags(id) ON DELETE CASCADE
);
CREATE INDEX idx_tag_assignments_media ON media_tag_assignments(media_id);
CREATE INDEX idx_tag_assignments_tag ON media_tag_assignments(tag_id);
```

**3. `media_thumbnails` - Thumbnail metadata**
```sql
CREATE TABLE media_thumbnails (
    media_id INTEGER PRIMARY KEY,
    thumbnail_path TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    generated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE
);
```

**4. `media_metadata` - Extended metadata (EXIF, video info, AI classifications)**
```sql
CREATE TABLE media_metadata (
    media_id INTEGER PRIMARY KEY,
    width INTEGER,
    height INTEGER,
    duration_seconds REAL,
    format TEXT,
    codec TEXT,
    ai_classifications TEXT, -- JSON array of detected objects/categories
    nsfw_score REAL,
    analyzed_at TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE
);
```

**5. FTS5 Virtual Table for Search**
```sql
CREATE VIRTUAL TABLE media_search_fts USING fts5(
    media_id UNINDEXED,
    post_title,
    community_name,
    creator_name,
    post_url,
    content='scraped_media',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER media_search_insert AFTER INSERT ON scraped_media BEGIN
    INSERT INTO media_search_fts(media_id, post_title, community_name, creator_name, post_url)
    VALUES (new.id, new.post_title, new.community_name, new.creator_name, new.post_url);
END;

CREATE TRIGGER media_search_delete AFTER DELETE ON scraped_media BEGIN
    DELETE FROM media_search_fts WHERE media_id = old.id;
END;

CREATE TRIGGER media_search_update AFTER UPDATE ON scraped_media BEGIN
    UPDATE media_search_fts
    SET post_title = new.post_title,
        community_name = new.community_name,
        creator_name = new.creator_name,
        post_url = new.post_url
    WHERE media_id = new.id;
END;
```

**6. `scraper_runs` - Track scraper execution for statistics**
```sql
CREATE TABLE scraper_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    posts_processed INTEGER DEFAULT 0,
    media_downloaded INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'running' -- running, completed, failed
);
CREATE INDEX idx_runs_started ON scraper_runs(started_at);
```

### New Go Packages/Files

#### 1. `internal/thumbnails/` - Thumbnail generation
- `generator.go` - Main thumbnail generation logic
- `video.go` - Video thumbnail extraction using ffmpeg
- `image.go` - Image thumbnail generation

#### 2. `internal/recognition/` - Image recognition
- `classifier.go` - Image classification interface
- `ollama.go` - Ollama integration for local AI
- `models.go` - Data structures for classification results

#### 3. `internal/tags/` - Tag management
- `manager.go` - Tag CRUD operations
- `auto_tagger.go` - Auto-tagging logic based on classifications

#### 4. `internal/search/` - Full-text search
- `search.go` - FTS5 search implementation
- `indexer.go` - Search index management

#### 5. `internal/stats/` - Statistics aggregation
- `stats.go` - Statistics calculation and caching
- `timeline.go` - Time-series data generation

#### 6. `internal/progress/` - Real-time progress tracking
- `tracker.go` - Progress state management
- `websocket.go` - WebSocket broadcasting

### API Endpoints

#### Search
- `GET /api/search?q=query&limit=50&offset=0` - Full-text search

#### Tags
- `GET /api/tags` - List all tags
- `POST /api/tags` - Create new tag
- `DELETE /api/tags/:id` - Delete tag
- `GET /api/media/:id/tags` - Get tags for media
- `POST /api/media/:id/tags` - Add tag to media
- `DELETE /api/media/:id/tags/:tagId` - Remove tag from media
- `POST /api/tags/auto-generate/:mediaId` - Trigger auto-tagging for media

#### Statistics
- `GET /api/stats/timeline?period=day|week|month` - Downloads over time
- `GET /api/stats/top-creators?limit=10` - Top contributors
- `GET /api/stats/storage` - Storage breakdown by community/type
- `GET /api/stats/distribution` - Media type distribution

#### Thumbnails
- `GET /thumbnails/{media_id}` - Serve thumbnail
- `POST /api/thumbnails/generate/:mediaId` - Generate thumbnail on-demand

#### Progress
- `WS /ws/progress` - WebSocket endpoint for real-time updates

### Frontend Components

#### Svelte Components (in `web/src/lib/components/`)

1. **SearchBar.svelte** - Search input with autocomplete
2. **SearchResults.svelte** - Search results grid
3. **TagManager.svelte** - Tag creation and management
4. **TagPill.svelte** - Individual tag display
5. **TagSelector.svelte** - Multi-select tag picker
6. **StatsTimeline.svelte** - Chart.js timeline chart
7. **StatsCards.svelte** - Statistics summary cards
8. **TopCreators.svelte** - Top creators list
9. **StorageBreakdown.svelte** - Storage pie chart
10. **ProgressBar.svelte** - Real-time progress indicator
11. **ProgressWebSocket.svelte** - WebSocket connection manager

#### New Routes (in `web/src/routes/`)

1. `/search/+page.svelte` - Search page
2. `/tags/+page.svelte` - Tag management page
3. `/stats/+page.svelte` - Statistics dashboard (enhance existing)

### Configuration Changes

Add to `internal/config/config.go`:

```go
type ThumbnailConfig struct {
    Enabled       bool   `yaml:"enabled"`
    MaxWidth      int    `yaml:"max_width"`
    MaxHeight     int    `yaml:"max_height"`
    Quality       int    `yaml:"quality"`
    Directory     string `yaml:"directory"`
    VideoMethod   string `yaml:"video_method"` // ffmpeg, frame_extract
}

type RecognitionConfig struct {
    Enabled        bool     `yaml:"enabled"`
    Provider       string   `yaml:"provider"` // ollama, none
    OllamaURL      string   `yaml:"ollama_url"`
    Model          string   `yaml:"model"` // llama3.2-vision, etc.
    AutoTag        bool     `yaml:"auto_tag"`
    NSFWDetection  bool     `yaml:"nsfw_detection"`
    Confidence     float64  `yaml:"confidence_threshold"`
}

type SearchConfig struct {
    RebuildIndex bool `yaml:"rebuild_index"`
}
```

Add to config.example.yaml:
```yaml
thumbnails:
  enabled: true
  max_width: 400
  max_height: 400
  quality: 85
  directory: "./thumbnails"
  video_method: "ffmpeg"

recognition:
  enabled: false
  provider: "ollama"
  ollama_url: "http://localhost:11434"
  model: "llama3.2-vision:latest"
  auto_tag: true
  nsfw_detection: false
  confidence_threshold: 0.6

search:
  rebuild_index: false
```

### Dependencies

#### Go Modules
```bash
go get github.com/disintegration/imaging  # Image processing
go get github.com/gorilla/websocket        # WebSocket support
go get github.com/ollama/ollama/api        # Ollama API client
```

#### NPM Packages
```bash
npm install --save chart.js svelte-chartjs  # Charts
npm install --save @tabler/icons-svelte     # Icons
```

## Implementation Order

### Phase 1: Database & Core Infrastructure
1. Database schema migrations
2. Add configuration options
3. Update models in `pkg/models/`

### Phase 2: Thumbnail Generation
1. Implement `internal/thumbnails/` package
2. Integrate with downloader
3. Add thumbnail API endpoints
4. Update media serving to prefer thumbnails

### Phase 3: Tags System
1. Implement `internal/tags/` package
2. Add tag API endpoints
3. Build tag management UI

### Phase 4: Image Recognition
1. Implement `internal/recognition/` package
2. Integrate Ollama client
3. Add auto-tagging logic
4. Create recognition API endpoints

### Phase 5: Full-Text Search
1. Implement `internal/search/` package
2. Create FTS5 indexes
3. Add search API endpoint
4. Build search UI

### Phase 6: Statistics Dashboard
1. Implement `internal/stats/` package
2. Add statistics API endpoints
3. Build Chart.js components
4. Create enhanced stats page

### Phase 7: Real-time Progress
1. Implement `internal/progress/` package
2. Add WebSocket endpoint
3. Integrate progress tracking in scraper
4. Build real-time UI components

### Phase 8: Integration & Testing
1. Wire all features together
2. End-to-end testing
3. Update documentation
4. Create PR

## Security Considerations

1. **Search** - Sanitize query input, limit results
2. **Tags** - Validate tag names, prevent XSS
3. **Thumbnails** - Path traversal protection (already implemented)
4. **WebSocket** - Rate limiting, authentication
5. **Image Recognition** - Validate Ollama responses, timeout protection

## Performance Considerations

1. **Thumbnails** - Generate asynchronously, cache aggressively
2. **FTS5** - Index maintenance, optimize queries
3. **WebSocket** - Limit broadcast frequency, connection limits
4. **Statistics** - Cache computed stats, periodic refresh
5. **Image Recognition** - Queue-based processing, timeout limits

## Testing Strategy

1. Unit tests for each new package
2. Integration tests for API endpoints
3. Manual testing of UI components
4. Performance testing with large datasets

## Documentation Updates

1. Update CLAUDE.md with new features
2. Update README.md with new configuration options
3. Add comments to example config
4. Create migration guide for database schema

## Rollback Plan

- Database migrations are versioned
- Features are disabled by default via config
- Can disable individual features without code changes
- Graceful degradation if dependencies unavailable (e.g., Ollama)
