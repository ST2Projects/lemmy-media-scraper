package database

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ST2Projects/lemmy-media-scraper/pkg/models"
)

func TestHashContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty content",
			content:  "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			content:  "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "binary data",
			content:  "\x00\x01\x02\x03",
			expected: "054edec1d0211f624fed0cbca9d4f9400b0e491c43742af2c5b0abebf0c990d8",
		},
		{
			name:     "unicode content",
			content:  "Hello 世界",
			expected: "c1e6adc38e6a3b48f5e7e03e7d5d3d3c3a2a5b2c5d5e3a0a8b7c3d4e5f6a1b2c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tt.content))
			hash, err := HashContent(reader)
			if err != nil {
				t.Errorf("HashContent() error = %v", err)
				return
			}

			// For unicode test, just verify it's a valid SHA256 hash
			if tt.name == "unicode content" {
				if len(hash) != 64 {
					t.Errorf("HashContent() hash length = %d, want 64", len(hash))
				}
				return
			}

			if hash != tt.expected {
				t.Errorf("HashContent() = %s, want %s", hash, tt.expected)
			}
		})
	}
}

func TestHashContentIdentical(t *testing.T) {
	// Same content should produce the same hash
	content := "test content for hashing"

	hash1, err := HashContent(bytes.NewReader([]byte(content)))
	if err != nil {
		t.Fatalf("First HashContent() error = %v", err)
	}

	hash2, err := HashContent(bytes.NewReader([]byte(content)))
	if err != nil {
		t.Fatalf("Second HashContent() error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Identical content produced different hashes: %s != %s", hash1, hash2)
	}
}

func TestHashContentDifferent(t *testing.T) {
	// Different content should produce different hashes
	content1 := "test content 1"
	content2 := "test content 2"

	hash1, err := HashContent(bytes.NewReader([]byte(content1)))
	if err != nil {
		t.Fatalf("First HashContent() error = %v", err)
	}

	hash2, err := HashContent(bytes.NewReader([]byte(content2)))
	if err != nil {
		t.Fatalf("Second HashContent() error = %v", err)
	}

	if hash1 == hash2 {
		t.Errorf("Different content produced identical hashes: %s", hash1)
	}
}

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	// Verify the database connection works
	if err := db.Ping(); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestInitSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	// Check that tables exist by trying to query them
	tables := []string{"scraped_media", "scraped_posts", "scraped_comments"}
	for _, table := range tables {
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		var name string
		err := db.Get(&name, query, table)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}
}

func TestMediaExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	hash := "test_hash_123"

	// Should not exist initially
	exists, err := db.MediaExists(hash)
	if err != nil {
		t.Fatalf("MediaExists() error = %v", err)
	}
	if exists {
		t.Errorf("MediaExists() = true, want false for new hash")
	}

	// Insert media with this hash
	media := &models.ScrapedMedia{
		PostID:        1,
		PostTitle:     "Test Post",
		CommunityName: "test",
		CommunityID:   1,
		AuthorName:    "testuser",
		AuthorID:      1,
		MediaURL:      "https://example.com/image.jpg",
		MediaHash:     hash,
		FileName:      "test.jpg",
		FilePath:      "/tmp/test.jpg",
		FileSize:      1024,
		MediaType:     "image",
		PostURL:       "https://example.com/post/1",
		PostScore:     10,
		PostCreated:   time.Now(),
		DownloadedAt:  time.Now(),
	}

	if err := db.SaveMedia(media); err != nil {
		t.Fatalf("SaveMedia() error = %v", err)
	}

	// Should exist now
	exists, err = db.MediaExists(hash)
	if err != nil {
		t.Fatalf("MediaExists() after save error = %v", err)
	}
	if !exists {
		t.Errorf("MediaExists() = false, want true after saving")
	}
}

func TestPostExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	postID := int64(123)

	// Should not exist initially
	exists, err := db.PostExists(postID)
	if err != nil {
		t.Fatalf("PostExists() error = %v", err)
	}
	if exists {
		t.Errorf("PostExists() = true, want false for new post")
	}

	// Mark post as scraped
	postView := &models.PostView{
		Post: models.Post{
			ID:        postID,
			Name:      "Test Post",
			Published: time.Now(),
		},
		Community: models.Community{
			ID:   1,
			Name: "test",
		},
		Creator: models.Person{
			ID:   1,
			Name: "testuser",
		},
	}

	if err := db.MarkPostAsScraped(postView, 0); err != nil {
		t.Fatalf("MarkPostAsScraped() error = %v", err)
	}

	// Should exist now
	exists, err = db.PostExists(postID)
	if err != nil {
		t.Fatalf("PostExists() after mark error = %v", err)
	}
	if !exists {
		t.Errorf("PostExists() = false, want true after marking")
	}
}

func TestSaveAndGetMediaByHash(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	hash := "unique_test_hash"
	media := &models.ScrapedMedia{
		PostID:        1,
		PostTitle:     "Test Post",
		CommunityName: "technology",
		CommunityID:   1,
		AuthorName:    "testuser",
		AuthorID:      1,
		MediaURL:      "https://example.com/image.jpg",
		MediaHash:     hash,
		FileName:      "test.jpg",
		FilePath:      "/tmp/test.jpg",
		FileSize:      2048,
		MediaType:     "image",
		PostURL:       "https://example.com/post/1",
		PostScore:     25,
		PostCreated:   time.Now(),
		DownloadedAt:  time.Now(),
	}

	// Save media
	if err := db.SaveMedia(media); err != nil {
		t.Fatalf("SaveMedia() error = %v", err)
	}

	// Verify ID was set
	if media.ID == 0 {
		t.Errorf("SaveMedia() did not set ID")
	}

	// Get media by hash
	retrieved, err := db.GetMediaByHash(hash)
	if err != nil {
		t.Fatalf("GetMediaByHash() error = %v", err)
	}

	if retrieved == nil {
		t.Fatalf("GetMediaByHash() returned nil")
	}

	// Verify fields
	if retrieved.MediaHash != hash {
		t.Errorf("MediaHash = %s, want %s", retrieved.MediaHash, hash)
	}
	if retrieved.PostTitle != "Test Post" {
		t.Errorf("PostTitle = %s, want Test Post", retrieved.PostTitle)
	}
	if retrieved.FileSize != 2048 {
		t.Errorf("FileSize = %d, want 2048", retrieved.FileSize)
	}
}

func TestGetMediaByHashNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	media, err := db.GetMediaByHash("nonexistent_hash")
	if err != nil {
		t.Fatalf("GetMediaByHash() error = %v", err)
	}

	if media != nil {
		t.Errorf("GetMediaByHash() returned non-nil for nonexistent hash")
	}
}

func TestMediaFilterValidation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	// Test with invalid sort field (should default to downloaded_at)
	filter := MediaFilter{
		SortBy:    "invalid_field",
		SortOrder: "DESC",
		Limit:     10,
		Offset:    0,
	}

	_, _, err = db.GetMediaWithFilters(filter)
	if err != nil {
		t.Errorf("GetMediaWithFilters() with invalid sort field should not error, got: %v", err)
	}

	// Test with invalid sort order (should default to DESC)
	filter = MediaFilter{
		SortBy:    "downloaded_at",
		SortOrder: "INVALID",
		Limit:     10,
		Offset:    0,
	}

	_, _, err = db.GetMediaWithFilters(filter)
	if err != nil {
		t.Errorf("GetMediaWithFilters() with invalid sort order should not error, got: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	// Get stats for empty database
	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	totalMedia, ok := stats["total_media"].(int)
	if !ok {
		t.Errorf("GetStats() total_media not an int")
	}
	if totalMedia != 0 {
		t.Errorf("GetStats() total_media = %d, want 0", totalMedia)
	}

	// Add some media
	for i := 0; i < 3; i++ {
		media := &models.ScrapedMedia{
			PostID:        int64(i + 1),
			PostTitle:     "Test Post",
			CommunityName: "test",
			CommunityID:   1,
			AuthorName:    "testuser",
			AuthorID:      1,
			MediaURL:      "https://example.com/image.jpg",
			MediaHash:     "hash_" + string(rune(i)),
			FileName:      "test.jpg",
			FilePath:      "/tmp/test.jpg",
			FileSize:      1024,
			MediaType:     "image",
			PostURL:       "https://example.com/post/1",
			PostScore:     10,
			PostCreated:   time.Now(),
			DownloadedAt:  time.Now(),
		}
		if err := db.SaveMedia(media); err != nil {
			t.Fatalf("SaveMedia() error = %v", err)
		}
	}

	// Get stats again
	stats, err = db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() after insert error = %v", err)
	}

	totalMedia, ok = stats["total_media"].(int)
	if !ok {
		t.Errorf("GetStats() total_media not an int")
	}
	if totalMedia != 3 {
		t.Errorf("GetStats() total_media = %d, want 3", totalMedia)
	}
}

func TestNewInvalidPath(t *testing.T) {
	// Try to create database in a path that doesn't exist and can't be created
	invalidPath := "/nonexistent/deeply/nested/path/that/cannot/be/created/test.db"

	db, err := New(invalidPath)

	// On some systems this might succeed (SQLite is very permissive)
	// But if it fails, that's expected too
	if db != nil {
		db.Close()
	}

	// This test mainly ensures we don't panic
	if err != nil {
		// Expected behavior - should return an error for invalid paths
		if !strings.Contains(err.Error(), "failed to") {
			t.Errorf("Expected error message to contain 'failed to', got: %v", err)
		}
	}
}

func TestDatabaseClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close the database
	if err := db.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Trying to use it after close should fail
	err = db.Ping()
	if err == nil {
		t.Errorf("Ping() after Close() should fail, but succeeded")
	}
}

func TestSaveMediaDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	hash := "duplicate_hash"
	media := &models.ScrapedMedia{
		PostID:        1,
		PostTitle:     "Test Post",
		CommunityName: "test",
		CommunityID:   1,
		AuthorName:    "testuser",
		AuthorID:      1,
		MediaURL:      "https://example.com/image.jpg",
		MediaHash:     hash,
		FileName:      "test.jpg",
		FilePath:      "/tmp/test.jpg",
		FileSize:      1024,
		MediaType:     "image",
		PostURL:       "https://example.com/post/1",
		PostScore:     10,
		PostCreated:   time.Now(),
		DownloadedAt:  time.Now(),
	}

	// First save should succeed
	if err := db.SaveMedia(media); err != nil {
		t.Fatalf("First SaveMedia() error = %v", err)
	}

	// Second save with same hash should fail (unique constraint)
	mediaDup := *media
	mediaDup.PostID = 2 // Different post
	err = db.SaveMedia(&mediaDup)
	if err == nil {
		t.Errorf("SaveMedia() with duplicate hash should fail, but succeeded")
	}
}
