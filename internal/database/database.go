package database

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/ST2Projects/lemmy-media-scraper/pkg/models"
	log "github.com/sirupsen/logrus"
)

// DB represents the database connection
type DB struct {
	*sqlx.DB
	ftsAvailable bool
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &DB{DB: db, ftsAvailable: false}
	if err := database.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

// initSchema creates the database tables if they don't exist
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS scraped_media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		post_title TEXT NOT NULL,
		community_name TEXT NOT NULL,
		community_id INTEGER NOT NULL,
		author_name TEXT NOT NULL,
		author_id INTEGER NOT NULL,
		media_url TEXT NOT NULL,
		media_hash TEXT NOT NULL UNIQUE,
		file_name TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		media_type TEXT NOT NULL,
		post_url TEXT NOT NULL,
		post_score INTEGER NOT NULL,
		post_created DATETIME NOT NULL,
		downloaded_at DATETIME NOT NULL,
		UNIQUE(post_id, media_url)
	);

	CREATE TABLE IF NOT EXISTS scraped_posts (
		post_id INTEGER PRIMARY KEY,
		post_title TEXT NOT NULL,
		community_name TEXT NOT NULL,
		community_id INTEGER NOT NULL,
		author_name TEXT NOT NULL,
		author_id INTEGER NOT NULL,
		post_created DATETIME NOT NULL,
		scraped_at DATETIME NOT NULL,
		had_media BOOLEAN NOT NULL,
		media_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS scraped_comments (
		comment_id INTEGER PRIMARY KEY,
		post_id INTEGER NOT NULL,
		creator_id INTEGER NOT NULL,
		creator_name TEXT NOT NULL,
		content TEXT NOT NULL,
		path TEXT NOT NULL,
		score INTEGER NOT NULL,
		upvotes INTEGER NOT NULL,
		downvotes INTEGER NOT NULL,
		child_count INTEGER NOT NULL,
		published DATETIME NOT NULL,
		updated DATETIME,
		removed BOOLEAN NOT NULL,
		deleted BOOLEAN NOT NULL,
		distinguished BOOLEAN NOT NULL,
		scraped_at DATETIME NOT NULL,
		FOREIGN KEY (post_id) REFERENCES scraped_posts(post_id)
	);

	CREATE INDEX IF NOT EXISTS idx_media_hash ON scraped_media(media_hash);
	CREATE INDEX IF NOT EXISTS idx_post_id ON scraped_media(post_id);
	CREATE INDEX IF NOT EXISTS idx_community_name ON scraped_media(community_name);
	CREATE INDEX IF NOT EXISTS idx_downloaded_at ON scraped_media(downloaded_at);
	CREATE INDEX IF NOT EXISTS idx_scraped_posts_community ON scraped_posts(community_name);
	CREATE INDEX IF NOT EXISTS idx_scraped_posts_scraped_at ON scraped_posts(scraped_at);
	CREATE INDEX IF NOT EXISTS idx_comments_post_id ON scraped_comments(post_id);
	CREATE INDEX IF NOT EXISTS idx_comments_path ON scraped_comments(path);

	-- Tags system
	CREATE TABLE IF NOT EXISTS media_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		color TEXT,
		auto_generated BOOLEAN DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tags_name ON media_tags(name);

	CREATE TABLE IF NOT EXISTS media_tag_assignments (
		media_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (media_id, tag_id),
		FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES media_tags(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tag_assignments_media ON media_tag_assignments(media_id);
	CREATE INDEX IF NOT EXISTS idx_tag_assignments_tag ON media_tag_assignments(tag_id);

	-- Thumbnails
	CREATE TABLE IF NOT EXISTS media_thumbnails (
		media_id INTEGER PRIMARY KEY,
		thumbnail_path TEXT NOT NULL,
		width INTEGER,
		height INTEGER,
		generated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE
	);

	-- Extended metadata
	CREATE TABLE IF NOT EXISTS media_metadata (
		media_id INTEGER PRIMARY KEY,
		width INTEGER,
		height INTEGER,
		duration_seconds REAL,
		format TEXT,
		codec TEXT,
		ai_classifications TEXT,
		nsfw_score REAL,
		analyzed_at TIMESTAMP,
		FOREIGN KEY (media_id) REFERENCES scraped_media(id) ON DELETE CASCADE
	);

	-- Scraper run tracking for statistics
	CREATE TABLE IF NOT EXISTS scraper_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		posts_processed INTEGER DEFAULT 0,
		media_downloaded INTEGER DEFAULT 0,
		errors_count INTEGER DEFAULT 0,
		status TEXT DEFAULT 'running'
	);

	CREATE INDEX IF NOT EXISTS idx_runs_started ON scraper_runs(started_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Initialize FTS5 search index (optional - gracefully fails if FTS5 not available)
	if err := db.initSearchIndex(); err != nil {
		// FTS5 might not be available in all SQLite builds
		// Log warning but don't fail initialization
		log.Warnf("FTS5 search index not available: %v", err)
		log.Warn("Full-text search will be disabled. To enable, rebuild SQLite with FTS5 support.")
		db.ftsAvailable = false
	} else {
		db.ftsAvailable = true
		log.Debug("FTS5 search index initialized successfully")
	}

	return nil
}

// MediaExists checks if media with the given hash already exists
func (db *DB) MediaExists(hash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM scraped_media WHERE media_hash = ?)`
	err := db.Get(&exists, query, hash)
	if err != nil {
		return false, fmt.Errorf("failed to check media existence: %w", err)
	}
	return exists, nil
}

// PostExists checks if a post has already been scraped
func (db *DB) PostExists(postID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM scraped_posts WHERE post_id = ?)`
	err := db.Get(&exists, query, postID)
	if err != nil {
		return false, fmt.Errorf("failed to check post existence: %w", err)
	}
	return exists, nil
}

// MarkPostAsScraped records that we've processed a post (with or without media)
func (db *DB) MarkPostAsScraped(postView *models.PostView, mediaCount int) error {
	query := `
		INSERT OR REPLACE INTO scraped_posts (
			post_id, post_title, community_name, community_id,
			author_name, author_id, post_created, scraped_at,
			had_media, media_count
		) VALUES (
			:post_id, :post_title, :community_name, :community_id,
			:author_name, :author_id, :post_created, datetime('now'),
			:had_media, :media_count
		)
	`

	params := map[string]interface{}{
		"post_id":        postView.Post.ID,
		"post_title":     postView.Post.Name,
		"community_name": postView.Community.Name,
		"community_id":   postView.Community.ID,
		"author_name":    postView.Creator.Name,
		"author_id":      postView.Creator.ID,
		"post_created":   postView.Post.Published,
		"had_media":      mediaCount > 0,
		"media_count":    mediaCount,
	}

	_, err := db.NamedExec(query, params)
	if err != nil {
		return fmt.Errorf("failed to mark post as scraped: %w", err)
	}

	return nil
}

// SaveMedia saves a scraped media record to the database
func (db *DB) SaveMedia(media *models.ScrapedMedia) error {
	query := `
		INSERT INTO scraped_media (
			post_id, post_title, community_name, community_id,
			author_name, author_id, media_url, media_hash,
			file_name, file_path, file_size, media_type,
			post_url, post_score, post_created, downloaded_at
		) VALUES (
			:post_id, :post_title, :community_name, :community_id,
			:author_name, :author_id, :media_url, :media_hash,
			:file_name, :file_path, :file_size, :media_type,
			:post_url, :post_score, :post_created, :downloaded_at
		)
	`

	result, err := db.NamedExec(query, media)
	if err != nil {
		return fmt.Errorf("failed to save media: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	media.ID = id
	return nil
}

// GetMediaByHash retrieves a media record by its hash
func (db *DB) GetMediaByHash(hash string) (*models.ScrapedMedia, error) {
	media := &models.ScrapedMedia{}
	query := `SELECT * FROM scraped_media WHERE media_hash = ?`

	err := db.Get(media, query, hash)
	if err != nil {
		// sqlx returns sql.ErrNoRows for Get() when no rows found
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get media by hash: %w", err)
	}

	return media, nil
}

// GetMediaByID retrieves a media record by its ID
func (db *DB) GetMediaByID(id int64) (*models.ScrapedMedia, error) {
	media := &models.ScrapedMedia{}
	query := `SELECT * FROM scraped_media WHERE id = ?`

	err := db.Get(media, query, id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, fmt.Errorf("media not found")
		}
		return nil, fmt.Errorf("failed to get media by ID: %w", err)
	}

	return media, nil
}

// GetStats returns statistics about scraped media
func (db *DB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total media count
	var totalCount int
	err := db.Get(&totalCount, `SELECT COUNT(*) FROM scraped_media`)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total_media"] = totalCount

	// Count by media type
	type TypeCount struct {
		MediaType string `db:"media_type"`
		Count     int    `db:"count"`
	}
	var typeCounts []TypeCount
	err = db.Select(&typeCounts, `SELECT media_type, COUNT(*) as count FROM scraped_media GROUP BY media_type`)
	if err != nil {
		return nil, fmt.Errorf("failed to get media type counts: %w", err)
	}

	typeMap := make(map[string]int)
	for _, tc := range typeCounts {
		typeMap[tc.MediaType] = tc.Count
	}
	stats["by_type"] = typeMap

	// Count by community
	type CommunityCount struct {
		CommunityName string `db:"community_name"`
		Count         int    `db:"count"`
	}
	var communityCounts []CommunityCount
	err = db.Select(&communityCounts, `SELECT community_name, COUNT(*) as count FROM scraped_media GROUP BY community_name ORDER BY count DESC LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("failed to get community counts: %w", err)
	}

	communityMap := make(map[string]int)
	for _, cc := range communityCounts {
		communityMap[cc.CommunityName] = cc.Count
	}
	stats["top_communities"] = communityMap

	return stats, nil
}

// HashContent computes the SHA256 hash of content
func HashContent(content io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, content); err != nil {
		return "", fmt.Errorf("failed to hash content: %w", err)
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// SaveComment saves a comment to the database
func (db *DB) SaveComment(commentView *models.CommentView) error {
	query := `
		INSERT OR REPLACE INTO scraped_comments (
			comment_id, post_id, creator_id, creator_name, content, path,
			score, upvotes, downvotes, child_count, published, updated,
			removed, deleted, distinguished, scraped_at
		) VALUES (
			:comment_id, :post_id, :creator_id, :creator_name, :content, :path,
			:score, :upvotes, :downvotes, :child_count, :published, :updated,
			:removed, :deleted, :distinguished, datetime('now')
		)
	`

	var updated interface{}
	if !commentView.Comment.Updated.IsZero() {
		updated = commentView.Comment.Updated
	}

	params := map[string]interface{}{
		"comment_id":    commentView.Comment.ID,
		"post_id":       commentView.Comment.PostID,
		"creator_id":    commentView.Creator.ID,
		"creator_name":  commentView.Creator.Name,
		"content":       commentView.Comment.Content,
		"path":          commentView.Comment.Path,
		"score":         commentView.Counts.Score,
		"upvotes":       commentView.Counts.Upvotes,
		"downvotes":     commentView.Counts.Downvotes,
		"child_count":   commentView.Counts.ChildCount,
		"published":     commentView.Comment.Published,
		"updated":       updated,
		"removed":       commentView.Comment.Removed,
		"deleted":       commentView.Comment.Deleted,
		"distinguished": commentView.Comment.Distinguished,
	}

	_, err := db.NamedExec(query, params)
	if err != nil {
		return fmt.Errorf("failed to save comment: %w", err)
	}

	return nil
}

// Comment represents a comment record from the database
type Comment struct {
	CommentID     int64  `db:"comment_id"`
	PostID        int64  `db:"post_id"`
	CreatorID     int64  `db:"creator_id"`
	CreatorName   string `db:"creator_name"`
	Content       string `db:"content"`
	Path          string `db:"path"`
	Score         int64  `db:"score"`
	Upvotes       int64  `db:"upvotes"`
	Downvotes     int64  `db:"downvotes"`
	ChildCount    int64  `db:"child_count"`
	Published     string `db:"published"`
	Updated       string `db:"updated"`
	Removed       bool   `db:"removed"`
	Deleted       bool   `db:"deleted"`
	Distinguished bool   `db:"distinguished"`
}

// GetCommentsByPostID retrieves all comments for a post, ordered by path for proper threading
func (db *DB) GetCommentsByPostID(postID int64) ([]map[string]interface{}, error) {
	query := `
		SELECT
			comment_id, post_id, creator_id, creator_name, content, path,
			score, upvotes, downvotes, child_count, published,
			COALESCE(updated, '') as updated,
			removed, deleted, distinguished
		FROM scraped_comments
		WHERE post_id = ? AND removed = 0 AND deleted = 0
		ORDER BY path ASC
	`

	var comments []Comment
	err := db.Select(&comments, query, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}

	// Convert to map format for backward compatibility with web UI
	result := make([]map[string]interface{}, len(comments))
	for i, c := range comments {
		result[i] = map[string]interface{}{
			"comment_id":    c.CommentID,
			"post_id":       c.PostID,
			"creator_id":    c.CreatorID,
			"creator_name":  c.CreatorName,
			"content":       c.Content,
			"path":          c.Path,
			"score":         c.Score,
			"upvotes":       c.Upvotes,
			"downvotes":     c.Downvotes,
			"child_count":   c.ChildCount,
			"published":     c.Published,
			"distinguished": c.Distinguished,
		}
		if c.Updated != "" {
			result[i]["updated"] = c.Updated
		}
	}

	return result, nil
}

// CommentsExistForPost checks if comments have been scraped for a post
func (db *DB) CommentsExistForPost(postID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM scraped_comments WHERE post_id = ? LIMIT 1)`
	err := db.Get(&exists, query, postID)
	if err != nil {
		return false, fmt.Errorf("failed to check comments existence: %w", err)
	}
	return exists, nil
}

// GetPostIDByMediaID retrieves the post ID for a media item
func (db *DB) GetPostIDByMediaID(mediaID int64) (int64, error) {
	var postID int64
	query := `SELECT post_id FROM scraped_media WHERE id = ?`
	err := db.Get(&postID, query, mediaID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, fmt.Errorf("media not found")
		}
		return 0, fmt.Errorf("failed to get post ID: %w", err)
	}
	return postID, nil
}

// MediaFilter represents filter options for querying media
type MediaFilter struct {
	Community string
	MediaType string
	SortBy    string
	SortOrder string
	Limit     int
	Offset    int
}

// GetMediaWithFilters retrieves media with optional filters
func (db *DB) GetMediaWithFilters(filter MediaFilter) ([]models.ScrapedMedia, int, error) {
	// Build query with filters
	query := `SELECT * FROM scraped_media`
	countQuery := `SELECT COUNT(*) FROM scraped_media`

	var whereClauses []string
	var args []interface{}

	if filter.Community != "" {
		whereClauses = append(whereClauses, "community_name = ?")
		args = append(args, filter.Community)
	}

	if filter.MediaType != "" {
		whereClauses = append(whereClauses, "media_type = ?")
		args = append(args, filter.MediaType)
	}

	// Add WHERE clause if needed
	if len(whereClauses) > 0 {
		whereClause := " WHERE " + strings.Join(whereClauses, " AND ")
		query += whereClause
		countQuery += whereClause
	}

	// Get total count
	var total int
	if err := db.Get(&total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

	// Add sorting and pagination
	allowedSortFields := map[string]bool{
		"downloaded_at": true,
		"post_created":  true,
		"file_size":     true,
		"post_score":    true,
	}

	sortBy := filter.SortBy
	if !allowedSortFields[sortBy] {
		sortBy = "downloaded_at"
	}

	sortOrder := filter.SortOrder
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", sortBy, sortOrder)
	args = append(args, filter.Limit, filter.Offset)

	// Execute query
	var media []models.ScrapedMedia
	if err := db.Select(&media, query, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to query media: %w", err)
	}

	return media, total, nil
}

// CommunityCount represents a community with its media count
type CommunityCount struct {
	Name  string `db:"community_name"`
	Count int    `db:"count"`
}

// GetCommunities returns a list of communities with their media counts
func (db *DB) GetCommunities() ([]CommunityCount, error) {
	query := `
		SELECT community_name, COUNT(*) as count
		FROM scraped_media
		GROUP BY community_name
		ORDER BY count DESC
	`

	var communities []CommunityCount
	err := db.Select(&communities, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query communities: %w", err)
	}

	return communities, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// initSearchIndex creates the FTS5 search index and triggers
func (db *DB) initSearchIndex() error {
	// Try to create FTS5 virtual table
	ftsSchema := `
		CREATE VIRTUAL TABLE IF NOT EXISTS media_search_fts USING fts5(
			media_id UNINDEXED,
			post_title,
			community_name,
			creator_name,
			post_url,
			content='scraped_media',
			content_rowid='id'
		);
	`

	if _, err := db.Exec(ftsSchema); err != nil {
		// FTS5 not available - this is not fatal, just disable search
		log.Warnf("FTS5 search index not available: %v", err)
		log.Warn("To enable search, rebuild with: go build -tags fts5 -o lemmy-scraper ./cmd/scraper")
		db.ftsAvailable = false
		return nil
	}

	// Create triggers to keep FTS in sync
	triggers := `
		CREATE TRIGGER IF NOT EXISTS media_search_insert AFTER INSERT ON scraped_media BEGIN
			INSERT INTO media_search_fts(rowid, media_id, post_title, community_name, creator_name, post_url)
			VALUES (new.id, new.id, new.post_title, new.community_name, new.author_name, new.post_url);
		END;

		CREATE TRIGGER IF NOT EXISTS media_search_delete AFTER DELETE ON scraped_media BEGIN
			DELETE FROM media_search_fts WHERE rowid = old.id;
		END;

		CREATE TRIGGER IF NOT EXISTS media_search_update AFTER UPDATE ON scraped_media BEGIN
			UPDATE media_search_fts
			SET post_title = new.post_title,
				community_name = new.community_name,
				creator_name = new.author_name,
				post_url = new.post_url
			WHERE rowid = new.id;
		END;
	`

	if _, err := db.Exec(triggers); err != nil {
		log.Warnf("Failed to create FTS triggers: %v", err)
		db.ftsAvailable = false
		return nil
	}

	// Check if we need to populate the FTS index
	// For FTS5 content tables, we check the source table and populate if needed
	var mediaCount int
	if err := db.Get(&mediaCount, "SELECT COUNT(*) FROM scraped_media"); err != nil {
		return fmt.Errorf("failed to check media count: %w", err)
	}

	// If there's existing media data, try to populate the FTS index
	// Using INSERT OR IGNORE to skip rows that already exist
	if mediaCount > 0 {
		log.Infof("Ensuring FTS5 search index is populated with %d media items...", mediaCount)
		rebuildQuery := `
			INSERT OR IGNORE INTO media_search_fts(rowid, media_id, post_title, community_name, creator_name, post_url)
			SELECT id, id, post_title, community_name, author_name, post_url FROM scraped_media;
		`
		if _, err := db.Exec(rebuildQuery); err != nil {
			log.Warnf("Failed to populate FTS index: %v", err)
			db.ftsAvailable = false
			return nil
		}
		log.Info("FTS5 search index ready")
	}

	// FTS5 is available and working
	db.ftsAvailable = true
	log.Info("FTS5 full-text search enabled")

	return nil
}

// SearchMedia performs full-text search across media
func (db *DB) SearchMedia(query string, limit int, offset int) ([]models.ScrapedMedia, int, error) {
	if query == "" {
		return []models.ScrapedMedia{}, 0, nil
	}

	// Check if FTS5 is available
	if !db.ftsAvailable {
		return nil, 0, fmt.Errorf("FTS5 search not available - SQLite build does not support FTS5")
	}

	// Count total results
	countQuery := `
		SELECT COUNT(*) FROM media_search_fts
		WHERE media_search_fts MATCH ?
	`
	var total int
	if err := db.Get(&total, countQuery, query); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Get search results
	// For FTS5 content tables, use rowid for joins
	searchQuery := `
		SELECT m.* FROM scraped_media m
		INNER JOIN media_search_fts fts ON m.id = fts.rowid
		WHERE media_search_fts MATCH ?
		ORDER BY fts.rank
		LIMIT ? OFFSET ?
	`

	var media []models.ScrapedMedia
	if err := db.Select(&media, searchQuery, query, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("failed to execute search: %w", err)
	}

	return media, total, nil
}

// Tag-related methods

// CreateTag creates a new tag
func (db *DB) CreateTag(name string, color string, autoGenerated bool) (int64, error) {
	query := `INSERT INTO media_tags (name, color, auto_generated) VALUES (?, ?, ?)`
	result, err := db.Exec(query, name, color, autoGenerated)
	if err != nil {
		return 0, fmt.Errorf("failed to create tag: %w", err)
	}
	return result.LastInsertId()
}

// GetAllTags retrieves all tags
func (db *DB) GetAllTags() ([]map[string]interface{}, error) {
	query := `SELECT id, name, color, auto_generated, created_at FROM media_tags ORDER BY name ASC`
	rows, err := db.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []map[string]interface{}
	for rows.Next() {
		tag := make(map[string]interface{})
		if err := rows.MapScan(tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetTagByID retrieves a tag by ID
func (db *DB) GetTagByID(tagID int64) (map[string]interface{}, error) {
	query := `SELECT id, name, color, auto_generated, created_at FROM media_tags WHERE id = ?`
	row := db.QueryRowx(query, tagID)

	tag := make(map[string]interface{})
	if err := row.MapScan(tag); err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	return tag, nil
}

// GetTagByName retrieves a tag by name
func (db *DB) GetTagByName(name string) (map[string]interface{}, error) {
	query := `SELECT id, name, color, auto_generated, created_at FROM media_tags WHERE name = ?`
	row := db.QueryRowx(query, name)

	tag := make(map[string]interface{})
	if err := row.MapScan(tag); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	return tag, nil
}

// DeleteTag deletes a tag
func (db *DB) DeleteTag(tagID int64) error {
	query := `DELETE FROM media_tags WHERE id = ?`
	_, err := db.Exec(query, tagID)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// AssignTagToMedia assigns a tag to a media item
func (db *DB) AssignTagToMedia(mediaID int64, tagID int64) error {
	query := `INSERT OR IGNORE INTO media_tag_assignments (media_id, tag_id) VALUES (?, ?)`
	_, err := db.Exec(query, mediaID, tagID)
	if err != nil {
		return fmt.Errorf("failed to assign tag: %w", err)
	}
	return nil
}

// RemoveTagFromMedia removes a tag from a media item
func (db *DB) RemoveTagFromMedia(mediaID int64, tagID int64) error {
	query := `DELETE FROM media_tag_assignments WHERE media_id = ? AND tag_id = ?`
	_, err := db.Exec(query, mediaID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag: %w", err)
	}
	return nil
}

// GetTagsForMedia retrieves all tags assigned to a media item
func (db *DB) GetTagsForMedia(mediaID int64) ([]map[string]interface{}, error) {
	query := `
		SELECT t.id, t.name, t.color, t.auto_generated, t.created_at
		FROM media_tags t
		INNER JOIN media_tag_assignments a ON t.id = a.tag_id
		WHERE a.media_id = ?
		ORDER BY t.name ASC
	`

	rows, err := db.Queryx(query, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []map[string]interface{}
	for rows.Next() {
		tag := make(map[string]interface{})
		if err := rows.MapScan(tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetUntaggedImages returns all image media IDs that have no tags
func (db *DB) GetUntaggedImages() ([]map[string]interface{}, error) {
	query := `
		SELECT m.id, m.file_path, m.post_title, m.community_name
		FROM scraped_media m
		LEFT JOIN media_tag_assignments a ON m.id = a.media_id
		WHERE a.media_id IS NULL
		AND (m.media_type = 'image' OR m.media_type LIKE 'image/%')
		ORDER BY m.downloaded_at DESC
	`

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query untagged images: %w", err)
	}
	defer rows.Close()

	var media []map[string]interface{}
	for rows.Next() {
		item := make(map[string]interface{})
		if err := rows.MapScan(item); err != nil {
			return nil, fmt.Errorf("failed to scan media: %w", err)
		}
		media = append(media, item)
	}

	return media, nil
}

// Thumbnail-related methods

// SaveThumbnail saves thumbnail metadata
func (db *DB) SaveThumbnail(mediaID int64, thumbnailPath string, width int, height int) error {
	query := `INSERT OR REPLACE INTO media_thumbnails (media_id, thumbnail_path, width, height, generated_at)
	          VALUES (?, ?, ?, ?, datetime('now'))`
	_, err := db.Exec(query, mediaID, thumbnailPath, width, height)
	if err != nil {
		return fmt.Errorf("failed to save thumbnail: %w", err)
	}
	return nil
}

// GetThumbnailPath retrieves the thumbnail path for a media item
func (db *DB) GetThumbnailPath(mediaID int64) (string, error) {
	var thumbnailPath string
	query := `SELECT thumbnail_path FROM media_thumbnails WHERE media_id = ?`
	err := db.Get(&thumbnailPath, query, mediaID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("failed to get thumbnail path: %w", err)
	}
	return thumbnailPath, nil
}

// SaveMetadata saves extended media metadata
func (db *DB) SaveMetadata(mediaID int64, metadata map[string]interface{}) error {
	query := `INSERT OR REPLACE INTO media_metadata
	          (media_id, width, height, duration_seconds, format, codec, ai_classifications, nsfw_score, analyzed_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`

	_, err := db.Exec(query,
		mediaID,
		metadata["width"],
		metadata["height"],
		metadata["duration_seconds"],
		metadata["format"],
		metadata["codec"],
		metadata["ai_classifications"],
		metadata["nsfw_score"],
	)
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	return nil
}

// GetMetadata retrieves extended metadata for a media item
func (db *DB) GetMetadata(mediaID int64) (map[string]interface{}, error) {
	query := `SELECT * FROM media_metadata WHERE media_id = ?`
	row := db.QueryRowx(query, mediaID)

	metadata := make(map[string]interface{})
	if err := row.MapScan(metadata); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	return metadata, nil
}

// Scraper run tracking methods

// StartScraperRun creates a new scraper run record
func (db *DB) StartScraperRun() (int64, error) {
	query := `INSERT INTO scraper_runs (status, started_at) VALUES ('running', datetime('now'))`
	result, err := db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to start scraper run: %w", err)
	}
	return result.LastInsertId()
}

// UpdateScraperRun updates a scraper run's progress
func (db *DB) UpdateScraperRun(runID int64, postsProcessed int, mediaDownloaded int, errorsCount int) error {
	query := `UPDATE scraper_runs SET posts_processed = ?, media_downloaded = ?, errors_count = ? WHERE id = ?`
	_, err := db.Exec(query, postsProcessed, mediaDownloaded, errorsCount, runID)
	if err != nil {
		return fmt.Errorf("failed to update scraper run: %w", err)
	}
	return nil
}

// CompleteScraperRun marks a scraper run as completed
func (db *DB) CompleteScraperRun(runID int64, status string) error {
	query := `UPDATE scraper_runs SET status = ?, completed_at = datetime('now') WHERE id = ?`
	_, err := db.Exec(query, status, runID)
	if err != nil {
		return fmt.Errorf("failed to complete scraper run: %w", err)
	}
	return nil
}

// GetRecentScraperRuns retrieves recent scraper runs for statistics
func (db *DB) GetRecentScraperRuns(limit int) ([]map[string]interface{}, error) {
	query := `SELECT * FROM scraper_runs ORDER BY started_at DESC LIMIT ?`
	rows, err := db.Queryx(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query scraper runs: %w", err)
	}
	defer rows.Close()

	var runs []map[string]interface{}
	for rows.Next() {
		run := make(map[string]interface{})
		if err := rows.MapScan(run); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// GetTimelineStats retrieves download statistics over time
func (db *DB) GetTimelineStats(period string) ([]map[string]interface{}, error) {
	var groupBy string
	switch period {
	case "hour":
		groupBy = "strftime('%Y-%m-%d %H:00', downloaded_at)"
	case "day":
		groupBy = "strftime('%Y-%m-%d', downloaded_at)"
	case "week":
		groupBy = "strftime('%Y-W%W', downloaded_at)"
	case "month":
		groupBy = "strftime('%Y-%m', downloaded_at)"
	default:
		groupBy = "strftime('%Y-%m-%d', downloaded_at)"
	}

	query := fmt.Sprintf(`
		SELECT %s as period,
		       COUNT(*) as count,
		       SUM(file_size) as total_bytes
		FROM scraped_media
		GROUP BY period
		ORDER BY period DESC
		LIMIT 100
	`, groupBy)

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		stat := make(map[string]interface{})
		if err := rows.MapScan(stat); err != nil {
			return nil, fmt.Errorf("failed to scan stat: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetTopCreators retrieves top content creators by media count
func (db *DB) GetTopCreators(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT author_name,
		       COUNT(*) as media_count,
		       SUM(post_score) as total_score,
		       MAX(downloaded_at) as last_download
		FROM scraped_media
		GROUP BY author_name
		ORDER BY media_count DESC
		LIMIT ?
	`

	rows, err := db.Queryx(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top creators: %w", err)
	}
	defer rows.Close()

	var creators []map[string]interface{}
	for rows.Next() {
		creator := make(map[string]interface{})
		if err := rows.MapScan(creator); err != nil {
			return nil, fmt.Errorf("failed to scan creator: %w", err)
		}
		creators = append(creators, creator)
	}

	return creators, nil
}

// GetStorageBreakdown retrieves storage usage by community and media type
func (db *DB) GetStorageBreakdown() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// By community
	communityQuery := `
		SELECT community_name, COUNT(*) as count, SUM(file_size) as total_bytes
		FROM scraped_media
		GROUP BY community_name
		ORDER BY total_bytes DESC
	`

	rows, err := db.Queryx(communityQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query community breakdown: %w", err)
	}
	defer rows.Close()

	var byCommunity []map[string]interface{}
	for rows.Next() {
		item := make(map[string]interface{})
		if err := rows.MapScan(item); err != nil {
			return nil, fmt.Errorf("failed to scan community: %w", err)
		}
		byCommunity = append(byCommunity, item)
	}
	result["by_community"] = byCommunity

	// By media type
	typeQuery := `
		SELECT media_type, COUNT(*) as count, SUM(file_size) as total_bytes
		FROM scraped_media
		GROUP BY media_type
		ORDER BY total_bytes DESC
	`

	rows2, err := db.Queryx(typeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query type breakdown: %w", err)
	}
	defer rows2.Close()

	var byType []map[string]interface{}
	for rows2.Next() {
		item := make(map[string]interface{})
		if err := rows2.MapScan(item); err != nil {
			return nil, fmt.Errorf("failed to scan type: %w", err)
		}
		byType = append(byType, item)
	}
	result["by_type"] = byType

	return result, nil
}
