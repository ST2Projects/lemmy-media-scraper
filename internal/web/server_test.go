package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ST2Projects/lemmy-media-scraper/internal/config"
	"github.com/ST2Projects/lemmy-media-scraper/internal/database"
	"github.com/ST2Projects/lemmy-media-scraper/internal/thumbnails"
	"github.com/ST2Projects/lemmy-media-scraper/pkg/models"
)

// setupTestServer creates a Server with a temp SQLite database for testing.
func setupTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mediaDir := filepath.Join(tmpDir, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("failed to create media dir: %v", err)
	}

	thumbDir := filepath.Join(tmpDir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		t.Fatalf("failed to create thumbnail dir: %v", err)
	}

	cfg := &config.Config{
		Lemmy: config.LemmyConfig{
			Instance: "lemmy.test",
			Username: "testuser",
			Password: "secret123",
		},
		Storage: config.StorageConfig{
			BaseDirectory: mediaDir,
		},
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
		Scraper: config.ScraperConfig{
			MaxPostsPerRun: 50,
			SortType:       "Hot",
			IncludeImages:  true,
			IncludeVideos:  true,
		},
		RunMode: config.RunModeConfig{
			Mode: "once",
		},
		WebServer: config.WebServerConfig{
			Enabled: true,
			Host:    "localhost",
			Port:    8080,
		},
		Thumbnails: config.ThumbnailConfig{
			Enabled:   true,
			MaxWidth:  400,
			MaxHeight: 400,
			Quality:   85,
			Directory: thumbDir,
		},
	}

	configPath := filepath.Join(tmpDir, "config.yaml")

	thumbGen := thumbnails.NewGenerator(400, 400, 85, thumbDir, "ffmpeg")

	s := New(cfg, configPath, db, nil, thumbGen)
	return s
}

// insertTestMedia inserts a test media record and returns it.
func insertTestMedia(t *testing.T, db *database.DB, postID int64, community string) *models.ScrapedMedia {
	t.Helper()
	media := &models.ScrapedMedia{
		PostID:        postID,
		PostTitle:     fmt.Sprintf("Test Post %d", postID),
		CommunityName: community,
		CommunityID:   1,
		AuthorName:    "testauthor",
		AuthorID:      1,
		MediaURL:      fmt.Sprintf("https://example.com/image_%d.jpg", postID),
		MediaHash:     fmt.Sprintf("hash_%d", postID),
		FileName:      fmt.Sprintf("image_%d.jpg", postID),
		FilePath:      fmt.Sprintf("/tmp/media/%s/image_%d.jpg", community, postID),
		FileSize:      1024,
		MediaType:     "image",
		PostURL:       fmt.Sprintf("https://lemmy.test/post/%d", postID),
		PostScore:     10,
		PostCreated:   time.Now(),
		DownloadedAt:  time.Now(),
	}

	if err := db.SaveMedia(media); err != nil {
		t.Fatalf("failed to insert test media: %v", err)
	}
	return media
}

func TestHandleGetMedia(t *testing.T) {
	s := setupTestServer(t)

	// Insert test data
	insertTestMedia(t, s.DB, 1, "pics")
	insertTestMedia(t, s.DB, 2, "pics")
	insertTestMedia(t, s.DB, 3, "videos")

	tests := []struct {
		name           string
		query          string
		wantStatus     int
		wantTotal      int
		checkMediaLen  bool
		wantMediaLen   int
	}{
		{
			name:          "default parameters",
			query:         "",
			wantStatus:    http.StatusOK,
			wantTotal:     3,
			checkMediaLen: true,
			wantMediaLen:  3,
		},
		{
			name:          "filter by community",
			query:         "?community=pics",
			wantStatus:    http.StatusOK,
			wantTotal:     2,
			checkMediaLen: true,
			wantMediaLen:  2,
		},
		{
			name:          "limit and offset",
			query:         "?limit=1&offset=0",
			wantStatus:    http.StatusOK,
			wantTotal:     3,
			checkMediaLen: true,
			wantMediaLen:  1,
		},
		{
			name:          "sort ascending",
			query:         "?sort=post_score&order=ASC",
			wantStatus:    http.StatusOK,
			wantTotal:     3,
			checkMediaLen: true,
			wantMediaLen:  3,
		},
		{
			name:       "no results for nonexistent community",
			query:      "?community=nonexistent",
			wantStatus: http.StatusOK,
			wantTotal:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/media"+tt.query, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			total := int(resp["total"].(float64))
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}

			if tt.checkMediaLen {
				media := resp["media"].([]interface{})
				if len(media) != tt.wantMediaLen {
					t.Errorf("media len = %d, want %d", len(media), tt.wantMediaLen)
				}
			}
		})
	}
}

func TestHandleGetMediaByID(t *testing.T) {
	s := setupTestServer(t)
	media := insertTestMedia(t, s.DB, 1, "pics")

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "valid ID",
			path:       fmt.Sprintf("/api/media/%d", media.ID),
			wantStatus: http.StatusOK,
		},
		{
			name:       "nonexistent ID",
			path:       "/api/media/9999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid ID",
			path:       "/api/media/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["post_title"] != "Test Post 1" {
					t.Errorf("post_title = %v, want 'Test Post 1'", resp["post_title"])
				}
			}
		})
	}
}

func TestHandleGetStats(t *testing.T) {
	s := setupTestServer(t)

	t.Run("empty database", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		totalMedia := int(resp["total_media"].(float64))
		if totalMedia != 0 {
			t.Errorf("total_media = %d, want 0", totalMedia)
		}
	})

	t.Run("with data", func(t *testing.T) {
		insertTestMedia(t, s.DB, 1, "pics")
		insertTestMedia(t, s.DB, 2, "pics")

		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		totalMedia := int(resp["total_media"].(float64))
		if totalMedia != 2 {
			t.Errorf("total_media = %d, want 2", totalMedia)
		}
	})
}

func TestHandleGetCommunities(t *testing.T) {
	s := setupTestServer(t)

	insertTestMedia(t, s.DB, 1, "pics")
	insertTestMedia(t, s.DB, 2, "pics")
	insertTestMedia(t, s.DB, 3, "videos")

	req := httptest.NewRequest(http.MethodGet, "/api/communities", nil)
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	communities := resp["communities"].([]interface{})
	if len(communities) != 2 {
		t.Errorf("communities len = %d, want 2", len(communities))
	}

	// First community should be pics (more media)
	first := communities[0].(map[string]interface{})
	if first["name"] != "pics" {
		t.Errorf("first community = %v, want 'pics'", first["name"])
	}
	if int(first["count"].(float64)) != 2 {
		t.Errorf("first community count = %v, want 2", first["count"])
	}
}

func TestHandleGetComments(t *testing.T) {
	s := setupTestServer(t)

	// Insert media first (comments reference post_id via media)
	media := insertTestMedia(t, s.DB, 100, "pics")

	// Mark the post as scraped so we can add comments
	postView := &models.PostView{
		Post: models.Post{
			ID:        100,
			Name:      "Test Post",
			Published: time.Now(),
		},
		Community: models.Community{ID: 1, Name: "pics"},
		Creator:   models.Person{ID: 1, Name: "testuser"},
	}
	if err := s.DB.MarkPostAsScraped(postView, 1); err != nil {
		t.Fatalf("failed to mark post: %v", err)
	}

	// Insert a comment
	commentView := &models.CommentView{
		Comment: models.Comment{
			ID:        1,
			PostID:    100,
			CreatorID: 1,
			Content:   "Great post!",
			Path:      "0.1",
			Published: time.Now(),
		},
		Creator: models.Person{ID: 1, Name: "commenter"},
		Counts:  models.CommentAggregates{Score: 5, Upvotes: 5, Downvotes: 0},
	}
	if err := s.DB.SaveComment(commentView); err != nil {
		t.Fatalf("failed to save comment: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "valid media ID with comments",
			path:       fmt.Sprintf("/api/comments/%d", media.ID),
			wantStatus: http.StatusOK,
		},
		{
			name:       "nonexistent media ID",
			path:       "/api/comments/9999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid media ID",
			path:       "/api/comments/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				comments := resp["comments"].([]interface{})
				if len(comments) != 1 {
					t.Errorf("comments len = %d, want 1", len(comments))
				}
			}
		})
	}
}

func TestHandleConfig(t *testing.T) {
	s := setupTestServer(t)

	t.Run("GET redacts password", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp config.Config
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Lemmy.Password != "" {
			t.Errorf("password should be empty/redacted, got %q", resp.Lemmy.Password)
		}
		if resp.Lemmy.Instance != "lemmy.test" {
			t.Errorf("instance = %q, want 'lemmy.test'", resp.Lemmy.Instance)
		}
	})

	t.Run("PUT with invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader("not json"))
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("PUT with valid config", func(t *testing.T) {
		newCfg := *s.Config
		newCfg.Lemmy.Password = "secret123" // needed for validation
		body, _ := json.Marshal(newCfg)

		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("unsupported method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/config", nil)
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestHandleServeMedia(t *testing.T) {
	s := setupTestServer(t)

	// Create a test media file
	communityDir := filepath.Join(s.Config.Storage.BaseDirectory, "pics")
	if err := os.MkdirAll(communityDir, 0755); err != nil {
		t.Fatalf("failed to create community dir: %v", err)
	}
	testFile := filepath.Join(communityDir, "test.jpg")
	if err := os.WriteFile(testFile, []byte("fake image data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "valid file",
			path:       "/media/pics/test.jpg",
			wantStatus: http.StatusOK,
		},
		{
			name:       "nonexistent file",
			path:       "/media/pics/missing.jpg",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "path traversal with dotdot",
			path:       "/media/../../../etc/passwd",
			wantStatus: http.StatusMovedPermanently, // Go HTTP router cleans .. and redirects
		},
		{
			name:       "path traversal encoded",
			path:       "/media/pics/..%2F..%2Fetc%2Fpasswd",
			wantStatus: http.StatusBadRequest, // handler catches cleaned path starting with ..
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleServeThumbnail(t *testing.T) {
	s := setupTestServer(t)

	// Insert media and its thumbnail record
	media := insertTestMedia(t, s.DB, 1, "pics")
	thumbPath := filepath.Join(s.Config.Thumbnails.Directory, "image_1.jpg")
	if err := os.WriteFile(thumbPath, []byte("fake thumb"), 0644); err != nil {
		t.Fatalf("failed to write thumbnail file: %v", err)
	}
	if err := s.DB.SaveThumbnail(media.ID, thumbPath, 200, 200); err != nil {
		t.Fatalf("failed to save thumbnail record: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "valid thumbnail",
			path:       fmt.Sprintf("/thumbnails/%d", media.ID),
			wantStatus: http.StatusOK,
		},
		{
			name:       "nonexistent media ID",
			path:       "/thumbnails/9999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid media ID",
			path:       "/thumbnails/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleSearch(t *testing.T) {
	s := setupTestServer(t)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "empty query returns empty results",
			query:      "/api/search",
			wantStatus: http.StatusOK,
		},
		{
			name:       "query too long",
			query:      "/api/search?q=" + strings.Repeat("a", 501),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-GET method",
			query:      "/api/search?q=test",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := http.MethodGet
			if tt.name == "non-GET method" {
				method = http.MethodPost
			}
			req := httptest.NewRequest(method, tt.query, nil)
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	s := setupTestServer(t)

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{
			name:       "allowed origin - SvelteKit dev",
			origin:     "http://localhost:5173",
			wantOrigin: "http://localhost:5173",
		},
		{
			name:       "allowed origin - API server",
			origin:     "http://localhost:8080",
			wantOrigin: "http://localhost:8080",
		},
		{
			name:       "disallowed origin",
			origin:     "http://evil.example.com",
			wantOrigin: "",
		},
		{
			name:       "no origin header",
			origin:     "",
			wantOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()

			s.handler.ServeHTTP(rec, req)

			gotOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if gotOrigin != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", gotOrigin, tt.wantOrigin)
			}
		})
	}

	t.Run("preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/stats", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()

		s.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("preflight status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	s := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expected := range expectedHeaders {
		got := rec.Header().Get(header)
		if got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}

	// Check that CSP and Permissions-Policy are present (don't need exact match)
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy header is missing")
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy header is missing")
	}
}
