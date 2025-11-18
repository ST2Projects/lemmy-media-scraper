package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ST2Projects/lemmy-media-scraper/internal/config"
	"github.com/ST2Projects/lemmy-media-scraper/internal/database"
	"github.com/ST2Projects/lemmy-media-scraper/internal/progress"
	"github.com/ST2Projects/lemmy-media-scraper/internal/tags"
	"github.com/ST2Projects/lemmy-media-scraper/internal/thumbnails"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Server represents the web server
type Server struct {
	Config           *config.Config
	ConfigPath       string
	DB               *database.DB
	ProgressTracker  *progress.Tracker
	TagManager       *tags.Manager
	ThumbnailGen     *thumbnails.Generator
	handler          http.Handler
	templates        *template.Template
	websocketUpgrader websocket.Upgrader
}

// New creates a new web server
func New(cfg *config.Config, configPath string, db *database.DB, progressTracker *progress.Tracker, tagManager *tags.Manager, thumbnailGen *thumbnails.Generator) *Server {
	s := &Server{
		Config:          cfg,
		ConfigPath:      configPath,
		DB:              db,
		ProgressTracker: progressTracker,
		TagManager:      tagManager,
		ThumbnailGen:    thumbnailGen,
		websocketUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now (can be restricted in production)
			},
		},
	}
	s.setupRoutes()
	return s
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS protection (legacy but still useful)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - restrictive but allows HTMX and inline styles
		csp := "default-src 'self'; " +
			"script-src 'self' https://unpkg.com 'unsafe-inline'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"media-src 'self'; " +
			"object-src 'none'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		w.Header().Set("Content-Security-Policy", csp)

		// Permissions Policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Parse embedded templates
	s.templates = template.Must(template.New("").Funcs(template.FuncMap{
		"formatFileSize": formatFileSize,
		"formatDate":     formatDate,
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}).Parse(indexTemplate + mediaGridTemplate + mediaModalTemplate + settingsTemplate + statsTemplate + tagsTemplate))

	mux := http.NewServeMux()

	// Main page
	mux.HandleFunc("/", s.handleIndex)

	// Stats and Tags pages
	mux.HandleFunc("/stats", s.handleStatsPage)
	mux.HandleFunc("/tags", s.handleTagsPage)

	// Settings page
	mux.HandleFunc("/settings", s.handleSettings)

	// HTMX endpoints
	mux.HandleFunc("/media-grid", s.handleMediaGrid)

	// API routes (kept for compatibility)
	mux.HandleFunc("/api/media/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a request for a specific media item (has ID after /api/media/)
		idPart := strings.TrimPrefix(r.URL.Path, "/api/media/")
		if idPart != "" && idPart != "/" {
			s.handleGetMediaByID(w, r)
			return
		}
		s.handleGetMedia(w, r)
	})
	mux.HandleFunc("/api/media", s.handleGetMedia)
	mux.HandleFunc("/api/stats", s.handleGetStats)
	mux.HandleFunc("/api/communities", s.handleGetCommunities)
	mux.HandleFunc("/api/comments/", s.handleGetComments)
	mux.HandleFunc("/api/config", s.handleConfig)

	// Search endpoints
	mux.HandleFunc("/api/search", s.handleSearch)

	// Tag endpoints
	mux.HandleFunc("/api/tags", s.handleTags)
	mux.HandleFunc("/api/tags/", s.handleTagByID)
	mux.HandleFunc("/api/media-tags/", s.handleMediaTags)
	mux.HandleFunc("/api/tags/backfill", s.handleTagBackfill)

	// Advanced statistics endpoints
	mux.HandleFunc("/api/stats/timeline", s.handleStatsTimeline)
	mux.HandleFunc("/api/stats/top-creators", s.handleStatsTopCreators)
	mux.HandleFunc("/api/stats/storage", s.handleStatsStorage)

	// WebSocket endpoint for real-time progress
	mux.HandleFunc("/ws/progress", s.handleWebSocket)

	// Serve media files and thumbnails
	mux.HandleFunc("/media/", s.handleServeMedia)
	mux.HandleFunc("/thumbnails/", s.handleServeThumbnail)

	// Wrap with security headers middleware
	s.handler = securityHeadersMiddleware(mux)
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.WebServer.Host, s.Config.WebServer.Port)
	log.Infof("Starting web server on http://%s", addr)
	return http.ListenAndServe(addr, s.handler)
}

// handleIndex serves the main HTML page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get initial data
	stats, _ := s.DB.GetStats()
	communities := s.getCommunityList()

	data := map[string]interface{}{
		"Stats":       stats,
		"Communities": communities,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index", data); err != nil {
		log.Errorf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleSettings serves the settings page
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "settings", nil); err != nil {
		log.Errorf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleMediaGrid serves the media grid (HTMX partial)
func (s *Server) handleMediaGrid(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse pagination
	limit := 50
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Parse filters
	searchQuery := query.Get("search")
	community := query.Get("community")
	mediaType := query.Get("type")
	sortBy := query.Get("sort")
	if sortBy == "" {
		sortBy = "downloaded_at"
	}
	sortOrder := query.Get("order")
	if sortOrder == "" {
		sortOrder = "DESC"
	}

	var media []map[string]interface{}
	var total int

	// Use search if query provided
	if searchQuery != "" {
		searchResults, searchTotal, err := s.DB.SearchMedia(searchQuery, limit, offset)
		if err != nil {
			log.Errorf("Search error: %v", err)
			media, total = s.getMediaList(community, mediaType, sortBy, sortOrder, limit, offset)
		} else {
			// Convert search results to map format
			media = make([]map[string]interface{}, len(searchResults))
			for i, m := range searchResults {
				media[i] = map[string]interface{}{
					"id":             m.ID,
					"post_id":        m.PostID,
					"post_title":     m.PostTitle,
					"community_name": m.CommunityName,
					"author_name":    m.AuthorName,
					"media_url":      m.MediaURL,
					"file_name":      m.FileName,
					"file_path":      m.FilePath,
					"file_size":      m.FileSize,
					"media_type":     m.MediaType,
					"post_url":       m.PostURL,
					"post_score":     m.PostScore,
					"post_created":   m.PostCreated,
					"downloaded_at":  m.DownloadedAt,
					"serve_url":      "/media/" + m.CommunityName + "/" + m.FileName,
				}
			}
			total = searchTotal
		}
	} else {
		media, total = s.getMediaList(community, mediaType, sortBy, sortOrder, limit, offset)
	}

	data := map[string]interface{}{
		"Media":      media,
		"Total":      total,
		"Limit":      limit,
		"Offset":     offset,
		"Community":  community,
		"Type":       mediaType,
		"Sort":       sortBy,
		"SortOrder":  sortOrder,
		"HasPrev":    offset > 0,
		"HasNext":    offset+limit < total,
		"Page":       (offset / limit) + 1,
		"TotalPages": (total + limit - 1) / limit,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "media-grid", data); err != nil {
		log.Errorf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleGetMedia returns a paginated list of media
func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse pagination params
	limit := 50
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Parse filter params
	sortBy := query.Get("sort")
	if sortBy == "" {
		sortBy = "downloaded_at"
	}

	sortOrder := query.Get("order")
	if sortOrder == "" {
		sortOrder = "DESC"
	}

	// Use database layer method for querying
	filter := database.MediaFilter{
		Community: query.Get("community"),
		MediaType: query.Get("type"),
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	mediaItems, total, err := s.DB.GetMediaWithFilters(filter)
	if err != nil {
		log.Errorf("Failed to get media: %v", err)
		http.Error(w, "Failed to query media", http.StatusInternalServerError)
		return
	}

	// Convert to map format for API response
	media := make([]map[string]interface{}, len(mediaItems))
	for i, item := range mediaItems {
		serveURL := fmt.Sprintf("/media/%s", filepath.Join(item.CommunityName, item.FileName))

		media[i] = map[string]interface{}{
			"id":             item.ID,
			"post_id":        item.PostID,
			"post_title":     item.PostTitle,
			"community_name": item.CommunityName,
			"community_id":   item.CommunityID,
			"author_name":    item.AuthorName,
			"author_id":      item.AuthorID,
			"media_url":      item.MediaURL,
			"media_hash":     item.MediaHash,
			"file_name":      item.FileName,
			"file_path":      item.FilePath,
			"file_size":      item.FileSize,
			"media_type":     item.MediaType,
			"post_url":       item.PostURL,
			"post_score":     item.PostScore,
			"post_created":   item.PostCreated.Format(time.RFC3339),
			"downloaded_at":  item.DownloadedAt.Format(time.RFC3339),
			"serve_url":      serveURL,
		}
	}

	response := map[string]interface{}{
		"media":  media,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetMediaByID returns a specific media item
func (s *Server) handleGetMediaByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/media/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	media, err := s.DB.GetMediaByID(id)
	if err != nil {
		if err.Error() == "media not found" {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		log.Errorf("Failed to get media by ID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	serveURL := fmt.Sprintf("/media/%s", filepath.Join(media.CommunityName, media.FileName))

	response := map[string]interface{}{
		"id":             media.ID,
		"post_id":        media.PostID,
		"post_title":     media.PostTitle,
		"community_name": media.CommunityName,
		"community_id":   media.CommunityID,
		"author_name":    media.AuthorName,
		"author_id":      media.AuthorID,
		"media_url":      media.MediaURL,
		"media_hash":     media.MediaHash,
		"file_name":      media.FileName,
		"file_path":      media.FilePath,
		"file_size":      media.FileSize,
		"media_type":     media.MediaType,
		"post_url":       media.PostURL,
		"post_score":     media.PostScore,
		"post_created":   media.PostCreated.Format(time.RFC3339),
		"downloaded_at":  media.DownloadedAt.Format(time.RFC3339),
		"serve_url":      serveURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetStats returns statistics about scraped media
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.DB.GetStats()
	if err != nil {
		log.Errorf("Failed to get stats: %v", err)
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleGetCommunities returns a list of communities with media counts
func (s *Server) handleGetCommunities(w http.ResponseWriter, r *http.Request) {
	communities, err := s.DB.GetCommunities()
	if err != nil {
		log.Errorf("Failed to query communities: %v", err)
		http.Error(w, "Failed to query communities", http.StatusInternalServerError)
		return
	}

	// Convert to map format for API response
	result := make([]map[string]interface{}, len(communities))
	for i, c := range communities {
		result[i] = map[string]interface{}{
			"name":  c.Name,
			"count": c.Count,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"communities": result,
	})
}

// handleGetComments returns comments for a specific media item's post
func (s *Server) handleGetComments(w http.ResponseWriter, r *http.Request) {
	// Extract media ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/comments/")
	mediaID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	// Get the post_id for this media item
	postID, err := s.DB.GetPostIDByMediaID(mediaID)
	if err != nil {
		if err.Error() == "media not found" {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		log.Errorf("Failed to get post ID for media: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get comments for the post
	comments, err := s.DB.GetCommentsByPostID(postID)
	if err != nil {
		log.Errorf("Failed to get comments: %v", err)
		http.Error(w, "Failed to get comments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"comments": comments,
		"post_id":  postID,
	})
}

// handleConfig handles GET and PUT requests for configuration management
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handleUpdateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetConfig returns the current configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return config without sensitive information (password)
	safeCfg := *s.Config
	safeCfg.Lemmy.Password = "" // Don't expose password

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeCfg)
}

// handleUpdateConfig updates the configuration and saves it to file
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If password is empty (not changed), keep the existing one
	if newConfig.Lemmy.Password == "" {
		newConfig.Lemmy.Password = s.Config.Lemmy.Password
	}

	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
		return
	}

	// Set defaults for any missing optional fields
	newConfig.SetDefaults()

	// Save to file
	if err := config.SaveConfig(s.ConfigPath, &newConfig); err != nil {
		log.Errorf("Failed to save config: %v", err)
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Update the in-memory config
	s.Config = &newConfig

	log.Info("Configuration updated successfully")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully. Restart the application for all changes to take effect.",
	})
}

// handleServeMedia serves media files from the storage directory
func (s *Server) handleServeMedia(w http.ResponseWriter, r *http.Request) {
	// Extract path after /media/
	mediaPath := strings.TrimPrefix(r.URL.Path, "/media/")

	// Prevent directory traversal - comprehensive protection
	// 1. Clean the path to resolve .. and . components
	cleanedPath := filepath.Clean(mediaPath)

	// 2. Reject absolute paths or paths starting with ..
	if filepath.IsAbs(cleanedPath) || strings.HasPrefix(cleanedPath, "..") {
		log.Warnf("Blocked path traversal attempt: %s", r.URL.Path)
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// 3. Construct full file path
	baseDir := filepath.Clean(s.Config.Storage.BaseDirectory)
	fullPath := filepath.Join(baseDir, cleanedPath)

	// 4. Ensure the resolved path is still within the base directory
	// This protects against symlink attacks and other bypasses
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// If we can't resolve symlinks, check if file exists first
		if _, statErr := os.Stat(fullPath); statErr != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		// File exists but we can't resolve symlinks - allow it
		resolvedPath = fullPath
	}

	// Ensure resolved path is within base directory
	if !strings.HasPrefix(resolvedPath, baseDir) {
		log.Warnf("Blocked access outside base directory: %s -> %s", r.URL.Path, resolvedPath)
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Serve the file
	http.ServeFile(w, r, resolvedPath)
}

// Helper functions

func (s *Server) getCommunityList() []map[string]interface{} {
	communities, err := s.DB.GetCommunities()
	if err != nil {
		return []map[string]interface{}{}
	}

	// Convert to map format for template compatibility
	result := make([]map[string]interface{}, len(communities))
	for i, c := range communities {
		result[i] = map[string]interface{}{
			"name":  c.Name,
			"count": c.Count,
		}
	}
	return result
}

func (s *Server) getMediaList(community, mediaType, sortBy, sortOrder string, limit, offset int) ([]map[string]interface{}, int) {
	// Use database layer method for querying
	filter := database.MediaFilter{
		Community: community,
		MediaType: mediaType,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	mediaItems, total, err := s.DB.GetMediaWithFilters(filter)
	if err != nil {
		log.Errorf("Failed to get media: %v", err)
		return []map[string]interface{}{}, 0
	}

	// Convert to map format for template compatibility
	media := make([]map[string]interface{}, len(mediaItems))
	for i, item := range mediaItems {
		serveURL := fmt.Sprintf("/media/%s", filepath.Join(item.CommunityName, item.FileName))

		media[i] = map[string]interface{}{
			"id":             item.ID,
			"post_id":        item.PostID,
			"post_title":     item.PostTitle,
			"community_name": item.CommunityName,
			"author_name":    item.AuthorName,
			"media_type":     item.MediaType,
			"file_size":      item.FileSize,
			"post_score":     item.PostScore,
			"post_url":       item.PostURL,
			"serve_url":      serveURL,
			"downloaded_at":  item.DownloadedAt.Format(time.RFC3339),
			"post_created":   item.PostCreated.Format(time.RFC3339),
		}
	}

	return media, total
}

func formatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

func formatDate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 2, 2006 3:04 PM")
}

// HTML Templates

const indexTemplate = `{{define "index"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lemmy Media Browser</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"
            integrity="sha384-D1Kt99CQMDuVetoL1lrYwg5t+9QdHe7NLX/SoJYkXDFfX37iInKRy5xLSi8nO7UC"
            crossorigin="anonymous"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #0f0f0f;
            color: #e0e0e0;
            line-height: 1.6;
        }
        .header {
            background: #1a1a1a;
            border-bottom: 1px solid #2a2a2a;
            padding: 12px 16px;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .header-content {
            max-width: 1400px;
            margin: 0 auto;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .header h1 { font-size: 24px; font-weight: 600; color: #fff; }
        .header-nav {
            display: flex;
            align-items: center;
            gap: 24px;
        }
        .header-nav a {
            color: #999;
            text-decoration: none;
            font-size: 14px;
            transition: color 0.2s;
        }
        .header-nav a:hover { color: #e0e0e0; }
        .stats {
            display: flex;
            gap: 24px;
            font-size: 14px;
            color: #999;
        }
        .stats span { font-weight: 600; color: #e0e0e0; }
        .filters {
            background: #1a1a1a;
            border-bottom: 1px solid #2a2a2a;
            padding: 8px 16px;
        }
        .filters-content {
            max-width: 1400px;
            margin: 0 auto;
            display: flex;
            gap: 12px;
            flex-wrap: wrap;
            align-items: center;
        }
        select, input[type="text"], input[type="search"] {
            background: #2a2a2a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
            padding: 6px 12px;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
        }
        select:hover, input:hover { background: #333; }
        input:focus { outline: none; border-color: #4a9eff; }
        .search-box {
            display: flex;
            gap: 8px;
            flex: 1;
            max-width: 400px;
        }
        .search-box input {
            flex: 1;
            min-width: 200px;
        }
        .btn {
            background: #4a9eff;
            color: #fff;
            border: none;
            padding: 6px 16px;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
            transition: background 0.2s;
        }
        .btn:hover { background: #3a8eef; }
        .btn-secondary {
            background: #2a2a2a;
            border: 1px solid #3a3a3a;
        }
        .btn-secondary:hover { background: #333; }
        .progress-bar {
            background: #2a2a2a;
            padding: 12px 16px;
            border-bottom: 1px solid #3a3a3a;
            display: none;
        }
        .progress-bar.active { display: block; }
        .progress-bar-content {
            max-width: 1400px;
            margin: 0 auto;
        }
        .progress-bar-fill {
            height: 4px;
            background: #4a9eff;
            border-radius: 2px;
            transition: width 0.3s;
        }
        .progress-info {
            display: flex;
            justify-content: space-between;
            font-size: 12px;
            color: #999;
            margin-top: 4px;
        }
        .tag {
            display: inline-block;
            background: #2a2a2a;
            color: #e0e0e0;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 11px;
            margin: 2px;
        }
        .tag.auto { opacity: 0.7; }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.9);
            z-index: 1000;
            padding: 20px;
            overflow-y: auto;
        }
        .modal.active { display: flex; }
        .modal-content {
            margin: auto;
            background: #1a1a1a;
            border-radius: 8px;
            padding: 24px;
            max-width: 600px;
            width: 100%;
        }
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        .modal-close {
            background: none;
            border: none;
            color: #999;
            font-size: 24px;
            cursor: pointer;
        }
        .modal-close:hover { color: #e0e0e0; }
        .content {
            max-width: 1400px;
            margin: 0 auto;
            padding: 24px 16px;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 12px;
        }
        @media (min-width: 640px) { .grid { grid-template-columns: repeat(2, 1fr); } }
        @media (min-width: 1024px) { .grid { grid-template-columns: repeat(4, 1fr); } }
        .card {
            background: #1a1a1a;
            border-radius: 8px;
            overflow: hidden;
            cursor: pointer;
            transition: all 0.2s;
        }
        .card:hover {
            transform: translateY(-4px);
            box-shadow: 0 8px 16px rgba(0,0,0,0.4);
        }
        .card-image {
            aspect-ratio: 4/3;
            background: #2a2a2a;
            position: relative;
            overflow: hidden;
        }
        .card-image img, .card-image video {
            width: 100%;
            height: 100%;
            object-fit: cover;
            transition: transform 0.2s;
        }
        .card:hover .card-image img, .card:hover .card-image video { transform: scale(1.05); }
        .card-image .play-overlay {
            position: absolute;
            inset: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            background: rgba(0, 0, 0, 0.3);
            pointer-events: none;
        }
        .card-image .play-overlay svg {
            width: 64px;
            height: 64px;
            fill: rgba(255, 255, 255, 0.9);
            filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));
        }
        .card-image .icon {
            position: absolute;
            inset: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .card-image .icon svg {
            width: 48px;
            height: 48px;
            fill: #666;
        }
        .card-info {
            padding: 12px;
        }
        .card-title {
            font-size: 14px;
            font-weight: 500;
            margin-bottom: 4px;
            overflow: hidden;
            text-overflow: ellipsis;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
        }
        .card-meta {
            font-size: 12px;
            color: #999;
            display: flex;
            gap: 8px;
            align-items: center;
        }
        .card-meta span:not(:last-child)::after {
            content: '•';
            margin-left: 8px;
        }
        .pagination {
            margin-top: 32px;
            padding-bottom: 32px;
            display: flex;
            justify-content: center;
            gap: 12px;
            align-items: center;
        }
        .btn {
            background: #2a2a2a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
            padding: 8px 16px;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
            transition: background 0.2s;
        }
        .btn:hover:not(:disabled) { background: #333; }
        .btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        .loading {
            text-align: center;
            padding: 64px;
            color: #999;
        }
        .modal {
            position: fixed;
            inset: 0;
            background: rgba(0,0,0,0.9);
            z-index: 1000;
            display: none;
            align-items: center;
            justify-content: center;
            padding: 16px;
        }
        .modal.active { display: flex; }
        .modal-content {
            background: #1a1a1a;
            border-radius: 8px;
            max-width: 1200px;
            max-height: 90vh;
            overflow: auto;
            position: relative;
        }
        .modal-header {
            padding: 16px;
            border-bottom: 1px solid #2a2a2a;
            display: flex;
            justify-content: space-between;
            align-items: start;
            position: sticky;
            top: 0;
            background: #1a1a1a;
            z-index: 10;
        }
        .modal-title { font-size: 18px; font-weight: 600; flex: 1; padding-right: 16px; }
        .modal-close {
            background: #2a2a2a;
            border: none;
            color: #e0e0e0;
            width: 32px;
            height: 32px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 20px;
        }
        .modal-close:hover { background: #333; }
        .modal-body { padding: 16px; }
        .modal-image {
            width: 100%;
            max-height: 70vh;
            object-fit: contain;
        }
        .modal-video {
            width: 100%;
            max-height: 70vh;
        }
        .modal-meta {
            margin-top: 16px;
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 16px;
            font-size: 14px;
            color: #999;
        }
        .modal-meta strong { color: #e0e0e0; }
        .modal-link {
            color: #4a9eff;
            text-decoration: none;
        }
        .modal-link:hover { text-decoration: underline; }
        .comments-section {
            margin-top: 24px;
            padding-top: 24px;
            border-top: 1px solid #2a2a2a;
        }
        .comments-header {
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 16px;
        }
        .comment {
            margin-bottom: 12px;
            padding: 12px;
            background: #2a2a2a;
            border-radius: 4px;
            border-left: 2px solid #3a3a3a;
        }
        .comment-nested {
            margin-left: 24px;
            margin-top: 8px;
            border-left-color: #4a4a4a;
        }
        .comment-header {
            display: flex;
            align-items: center;
            gap: 8px;
            margin-bottom: 8px;
            font-size: 13px;
        }
        .comment-author {
            font-weight: 600;
            color: #4a9eff;
        }
        .comment-score {
            color: #999;
        }
        .comment-score.positive { color: #ff6b35; }
        .comment-time {
            color: #666;
            font-size: 12px;
        }
        .comment-content {
            font-size: 14px;
            line-height: 1.5;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .comment-distinguished {
            background: #1a3a1a;
            border-left-color: #2a5a2a;
        }
        .loading-comments {
            text-align: center;
            padding: 24px;
            color: #999;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>Lemmy Media</h1>
            <div class="header-nav">
                <div class="stats">
                    {{if .Stats.total_media}}
                        <div><span>{{.Stats.total_media}}</span> items</div>
                        {{range $type, $count := .Stats.by_type}}
                            <div><span>{{$count}}</span> {{$type}}</div>
                        {{end}}
                    {{end}}
                </div>
                <a href="/stats">📊 Stats</a>
                <a href="/tags">🏷️ Tags</a>
                <a href="/settings">⚙️ Settings</a>
            </div>
        </div>
    </div>

    <!-- Real-time Progress Bar -->
    <div id="progress-bar" class="progress-bar">
        <div class="progress-bar-content">
            <div class="progress-bar-fill" id="progress-fill" style="width: 0%"></div>
            <div class="progress-info">
                <span id="progress-text">Idle</span>
                <span id="progress-eta"></span>
            </div>
        </div>
    </div>

    <div class="filters">
        <div class="filters-content">
            <!-- Search Box -->
            <div class="search-box">
                <input type="search"
                       id="search"
                       name="search"
                       placeholder="Search media..."
                       hx-get="/media-grid"
                       hx-trigger="keyup changed delay:500ms, search"
                       hx-target="#media-container"
                       hx-include="[name='community'],[name='type'],[name='sort'],[name='order']">
                <button class="btn btn-secondary" onclick="document.getElementById('search').value=''; document.body.dispatchEvent(new CustomEvent('filterChange'));">Clear</button>
            </div>

            <select id="community" name="community">
                <option value="">All Communities</option>
                {{range .Communities}}
                    <option value="{{.name}}">{{.name}} ({{.count}})</option>
                {{end}}
            </select>
            <select id="type" name="type">
                <option value="">All Types</option>
                <option value="image">Images</option>
                <option value="video">Videos</option>
                <option value="other">Other</option>
            </select>
            <select id="sort" name="sort">
                <option value="downloaded_at">Downloaded</option>
                <option value="post_created">Posted</option>
                <option value="file_size">File Size</option>
                <option value="post_score">Score</option>
            </select>
            <select id="order" name="order">
                <option value="DESC">Newest</option>
                <option value="ASC">Oldest</option>
            </select>
        </div>
    </div>

    <div class="content">
        <div id="media-container"
             hx-get="/media-grid"
             hx-trigger="load, filterChange from:body"
             hx-include="[name='community'],[name='type'],[name='sort'],[name='order']">
            <div class="loading">Loading...</div>
        </div>
    </div>

    <div id="modal" class="modal" onclick="if(event.target === this) this.classList.remove('active')">
        <div class="modal-content" onclick="event.stopPropagation()">
            <div id="modal-body"></div>
        </div>
    </div>

    <script>
        // Trigger filter updates
        document.querySelectorAll('select').forEach(select => {
            select.addEventListener('change', () => {
                document.body.dispatchEvent(new CustomEvent('filterChange'));
            });
        });

        // Modal functions
        window.openModal = function(id) {
            fetch('/api/media/' + id)
                .then(r => r.json())
                .then(item => {
                    if (item) {
                        showModal(item);
                    }
                });
        };

        function showModal(item) {
            // Validate URLs to prevent XSS in href/src attributes
            const safeServeUrl = sanitizeUrl(item.serve_url);
            const safePostUrl = sanitizeUrl(item.post_url);

            let mediaHTML = '';
            if (item.media_type === 'image') {
                mediaHTML = '<img src="' + safeServeUrl + '" class="modal-image" alt="' + escapeHtml(item.post_title) + '">';
            } else if (item.media_type === 'video') {
                mediaHTML = '<video src="' + safeServeUrl + '" class="modal-video" controls></video>';
            } else {
                mediaHTML = '<div style="text-align:center;padding:32px;">Preview not available. <a href="' + safeServeUrl + '" class="modal-link" download>Download</a></div>';
            }

            document.getElementById('modal-body').innerHTML =
                '<div class="modal-header">' +
                    '<div class="modal-title">' + escapeHtml(item.post_title) + '</div>' +
                    '<button class="modal-close" onclick="document.getElementById(\'modal\').classList.remove(\'active\')">&times;</button>' +
                '</div>' +
                '<div class="modal-body">' +
                    mediaHTML +
                    '<div class="modal-meta">' +
                        '<div><strong>Author:</strong> ' + escapeHtml(item.author_name) + '</div>' +
                        '<div><strong>Community:</strong> ' + escapeHtml(item.community_name) + '</div>' +
                        '<div><strong>Score:</strong> ' + escapeHtml(String(item.post_score)) + '</div>' +
                        '<div><strong>Type:</strong> ' + escapeHtml(item.media_type) + '</div>' +
                        '<div style="grid-column: 1/-1"><strong>Post:</strong> <a href="' + safePostUrl + '" target="_blank" rel="noopener noreferrer" class="modal-link">' + escapeHtml(item.post_url) + '</a></div>' +
                    '</div>' +
                    '<div class="comments-section" id="comments-section">' +
                        '<div class="loading-comments">Loading comments...</div>' +
                    '</div>' +
                '</div>';

            document.getElementById('modal').classList.add('active');

            // Fetch and display comments
            loadComments(item.id);
        }

        function loadComments(mediaId) {
            fetch('/api/comments/' + mediaId)
                .then(r => r.json())
                .then(data => {
                    displayComments(data.comments || []);
                })
                .catch(err => {
                    document.getElementById('comments-section').innerHTML =
                        '<div class="loading-comments">Failed to load comments</div>';
                });
        }

        function displayComments(comments) {
            const section = document.getElementById('comments-section');

            if (comments.length === 0) {
                section.innerHTML = '<div class="comments-header">No comments yet</div>';
                return;
            }

            // Build comment tree based on path
            const commentTree = buildCommentTree(comments);

            section.innerHTML = '<div class="comments-header">' + comments.length + ' Comment' + (comments.length === 1 ? '' : 's') + '</div>' +
                renderCommentTree(commentTree);
        }

        function buildCommentTree(comments) {
            // Sort by path to ensure proper ordering
            comments.sort((a, b) => a.path.localeCompare(b.path));
            return comments;
        }

        function renderCommentTree(comments) {
            let html = '';
            const pathDepthMap = {};

            for (const comment of comments) {
                const depth = (comment.path.match(/\./g) || []).length;
                const nestClass = depth > 0 ? 'comment-nested' : '';
                const distClass = comment.distinguished ? 'comment-distinguished' : '';
                const scoreClass = comment.score > 0 ? 'positive' : '';

                const timeAgo = formatTimeAgo(comment.published);

                html += '<div class="comment ' + nestClass + ' ' + distClass + '" style="margin-left: ' + (depth * 24) + 'px;">' +
                    '<div class="comment-header">' +
                        '<span class="comment-author">' + escapeHtml(comment.creator_name) + '</span>' +
                        '<span class="comment-score ' + scoreClass + '">↑ ' + comment.score + '</span>' +
                        '<span class="comment-time">' + timeAgo + '</span>' +
                    '</div>' +
                    '<div class="comment-content">' + escapeHtml(comment.content) + '</div>' +
                '</div>';
            }

            return html;
        }

        function formatTimeAgo(dateStr) {
            const date = new Date(dateStr);
            const now = new Date();
            const seconds = Math.floor((now - date) / 1000);

            if (seconds < 60) return seconds + 's ago';
            if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
            if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
            if (seconds < 2592000) return Math.floor(seconds / 86400) + 'd ago';
            return Math.floor(seconds / 2592000) + 'mo ago';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function sanitizeUrl(url) {
            // Prevent javascript: and data: URLs for XSS protection
            if (!url) return '';

            const urlLower = url.toLowerCase().trim();

            // Block dangerous URL schemes
            const dangerousSchemes = ['javascript:', 'data:', 'vbscript:', 'file:'];
            for (const scheme of dangerousSchemes) {
                if (urlLower.startsWith(scheme)) {
                    console.warn('Blocked dangerous URL scheme:', url);
                    return '#';
                }
            }

            // Only allow http, https, and relative URLs
            if (urlLower.startsWith('http://') || urlLower.startsWith('https://') || url.startsWith('/')) {
                return url;
            }

            // Relative URLs without leading slash
            if (!url.includes(':')) {
                return url;
            }

            // Unknown scheme - block it
            console.warn('Blocked unknown URL scheme:', url);
            return '#';
        }

        // WebSocket for real-time progress updates
        (function() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/progress';
            let ws = null;
            let reconnectTimeout = null;

            function connect() {
                try {
                    ws = new WebSocket(wsUrl);

                    ws.onopen = function() {
                        console.log('Progress WebSocket connected');
                    };

                    ws.onmessage = function(event) {
                        try {
                            const status = JSON.parse(event.data);
                            updateProgress(status);
                        } catch (e) {
                            console.error('Failed to parse progress update:', e);
                        }
                    };

                    ws.onerror = function(error) {
                        console.error('WebSocket error:', error);
                    };

                    ws.onclose = function() {
                        console.log('Progress WebSocket disconnected');
                        // Reconnect after 5 seconds
                        reconnectTimeout = setTimeout(connect, 5000);
                    };
                } catch (e) {
                    console.error('WebSocket connection failed:', e);
                    reconnectTimeout = setTimeout(connect, 5000);
                }
            }

            function updateProgress(status) {
                const progressBar = document.getElementById('progress-bar');
                const progressFill = document.getElementById('progress-fill');
                const progressText = document.getElementById('progress-text');
                const progressEta = document.getElementById('progress-eta');

                if (status.is_running) {
                    progressBar.classList.add('active');
                    progressFill.style.width = status.progress + '%';

                    let text = status.current_operation || 'Processing...';
                    text += ' (' + status.posts_processed + ' posts, ' + status.media_downloaded + ' media';
                    if (status.errors_count > 0) {
                        text += ', ' + status.errors_count + ' errors';
                    }
                    text += ')';
                    progressText.textContent = text;

                    if (status.eta) {
                        progressEta.textContent = 'ETA: ' + status.eta;
                    } else {
                        progressEta.textContent = '';
                    }
                } else {
                    progressBar.classList.remove('active');
                }
            }

            // Start connection
            connect();

            // Cleanup on page unload
            window.addEventListener('beforeunload', function() {
                if (ws) ws.close();
                if (reconnectTimeout) clearTimeout(reconnectTimeout);
            });
        })();
    </script>
</body>
</html>
{{end}}`

const mediaGridTemplate = `{{define "media-grid"}}
<div class="grid">
    {{range .Media}}
    <div class="card" onclick="openModal({{.id}})">
        <div class="card-image">
            {{if eq .media_type "image"}}
                <img src="{{.serve_url}}" alt="{{.post_title}}" loading="lazy">
            {{else if eq .media_type "video"}}
                <video src="{{.serve_url}}" preload="metadata" muted playsinline loading="lazy"></video>
                <div class="play-overlay">
                    <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                </div>
            {{else}}
                <div class="icon">
                    <svg viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd"/></svg>
                </div>
            {{end}}
        </div>
        <div class="card-info">
            <div class="card-title" title="{{.post_title}}">{{.post_title}}</div>
            <div class="card-meta">
                <span>{{.community_name}}</span>
                <span>{{.post_score}} pts</span>
                <span>{{.media_type}}</span>
            </div>
        </div>
    </div>
    {{end}}
</div>

{{if or .HasPrev .HasNext}}
<div class="pagination">
    <button class="btn"
            {{if .HasPrev}}
            hx-get="/media-grid?offset={{sub .Offset .Limit}}&limit={{.Limit}}&community={{.Community}}&type={{.Type}}&sort={{.Sort}}&order={{.SortOrder}}"
            hx-target="#media-container"
            {{else}}disabled{{end}}>
        ← Previous
    </button>
    <span style="color: #999; font-size: 14px;">Page {{.Page}} of {{.TotalPages}}</span>
    <button class="btn"
            {{if .HasNext}}
            hx-get="/media-grid?offset={{add .Offset .Limit}}&limit={{.Limit}}&community={{.Community}}&type={{.Type}}&sort={{.Sort}}&order={{.SortOrder}}"
            hx-target="#media-container"
            {{else}}disabled{{end}}>
        Next →
    </button>
</div>
{{end}}
{{end}}`

const mediaModalTemplate = ``

const settingsTemplate = `{{define "settings"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Settings - Lemmy Media Browser</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"
            integrity="sha384-D1Kt99CQMDuVetoL1lrYwg5t+9QdHe7NLX/SoJYkXDFfX37iInKRy5xLSi8nO7UC"
            crossorigin="anonymous"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #0f0f0f;
            color: #e0e0e0;
            line-height: 1.6;
        }
        .header {
            background: #1a1a1a;
            border-bottom: 1px solid #2a2a2a;
            padding: 12px 16px;
        }
        .header-content {
            max-width: 1000px;
            margin: 0 auto;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .header h1 { font-size: 24px; font-weight: 600; color: #fff; }
        .back-link {
            color: #4a9eff;
            text-decoration: none;
            font-size: 14px;
        }
        .back-link:hover { text-decoration: underline; }
        .content {
            max-width: 1000px;
            margin: 0 auto;
            padding: 32px 16px;
        }
        .section {
            background: #1a1a1a;
            border-radius: 8px;
            padding: 24px;
            margin-bottom: 24px;
        }
        .section-title {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 16px;
            color: #fff;
        }
        .form-group {
            margin-bottom: 20px;
        }
        .form-group label {
            display: block;
            font-size: 14px;
            font-weight: 500;
            margin-bottom: 6px;
            color: #e0e0e0;
        }
        .form-group .help-text {
            font-size: 12px;
            color: #999;
            margin-top: 4px;
        }
        input[type="text"],
        input[type="password"],
        input[type="number"],
        select,
        textarea {
            width: 100%;
            background: #2a2a2a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
            padding: 10px 12px;
            border-radius: 6px;
            font-size: 14px;
            font-family: inherit;
        }
        input:focus,
        select:focus,
        textarea:focus {
            outline: none;
            border-color: #4a9eff;
        }
        textarea {
            min-height: 100px;
            resize: vertical;
        }
        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .checkbox-group input[type="checkbox"] {
            width: auto;
            cursor: pointer;
        }
        .btn {
            background: #4a9eff;
            color: #fff;
            border: none;
            padding: 10px 24px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.2s;
        }
        .btn:hover { background: #3a8eef; }
        .btn:disabled {
            background: #2a2a2a;
            color: #666;
            cursor: not-allowed;
        }
        .btn-secondary {
            background: #2a2a2a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
        }
        .btn-secondary:hover { background: #333; }
        .alert {
            padding: 12px 16px;
            border-radius: 6px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        .alert-success {
            background: #1a3a1a;
            border: 1px solid #2a5a2a;
            color: #6fd46f;
        }
        .alert-error {
            background: #3a1a1a;
            border: 1px solid #5a2a2a;
            color: #f46f6f;
        }
        .alert-warning {
            background: #3a3a1a;
            border: 1px solid #5a5a2a;
            color: #f4d46f;
        }
        .form-actions {
            display: flex;
            gap: 12px;
            margin-top: 24px;
        }
        .loading {
            text-align: center;
            padding: 32px;
            color: #999;
        }
        .communities-input {
            font-family: 'Courier New', monospace;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>Settings</h1>
            <a href="/" class="back-link">← Back to Media</a>
        </div>
    </div>

    <div class="content">
        <div id="alert-container"></div>

        <form id="config-form">
            <div class="section">
                <div class="section-title">Lemmy Instance</div>
                <div class="form-group">
                    <label for="instance">Instance</label>
                    <input type="text" id="instance" name="instance" placeholder="lemmy.ml" required>
                    <div class="help-text">The Lemmy instance domain (e.g., lemmy.ml, lemmy.world)</div>
                </div>
                <div class="form-group">
                    <label for="username">Username</label>
                    <input type="text" id="username" name="username" required>
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" placeholder="Leave empty to keep current">
                    <div class="help-text">Leave empty to keep the current password</div>
                </div>
                <div class="form-group">
                    <label for="communities">Communities</label>
                    <textarea id="communities" name="communities" class="communities-input" placeholder='["technology", "linux"]'></textarea>
                    <div class="help-text">JSON array of community names. Empty array [] scrapes from instance hot page.</div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Storage</div>
                <div class="form-group">
                    <label for="base_directory">Base Directory</label>
                    <input type="text" id="base_directory" name="base_directory" placeholder="./downloads" required>
                    <div class="help-text">Where to save downloaded media files</div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Database</div>
                <div class="form-group">
                    <label for="database_path">Database Path</label>
                    <input type="text" id="database_path" name="database_path" placeholder="./scraper.db" required>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Scraper Settings</div>
                <div class="form-group">
                    <label for="max_posts_per_run">Max Posts Per Run</label>
                    <input type="number" id="max_posts_per_run" name="max_posts_per_run" min="1" required>
                    <div class="help-text">Maximum number of posts to scrape per run</div>
                </div>
                <div class="form-group">
                    <label for="sort_type">Sort Type</label>
                    <select id="sort_type" name="sort_type">
                        <option value="Hot">Hot</option>
                        <option value="New">New</option>
                        <option value="TopDay">Top Day</option>
                        <option value="TopWeek">Top Week</option>
                        <option value="TopMonth">Top Month</option>
                        <option value="TopYear">Top Year</option>
                        <option value="TopAll">Top All</option>
                        <option value="Active">Active</option>
                    </select>
                </div>
                <div class="form-group">
                    <label for="seen_posts_threshold">Seen Posts Threshold</label>
                    <input type="number" id="seen_posts_threshold" name="seen_posts_threshold" min="1">
                    <div class="help-text">Stop after encountering this many consecutive seen posts</div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="stop_at_seen_posts" name="stop_at_seen_posts">
                        <label for="stop_at_seen_posts">Stop at seen posts</label>
                    </div>
                    <div class="help-text">Stop scraping after hitting threshold of consecutive seen posts</div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="skip_seen_posts" name="skip_seen_posts">
                        <label for="skip_seen_posts">Skip seen posts</label>
                    </div>
                    <div class="help-text">Skip seen posts but continue scraping (use with caution)</div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="enable_pagination" name="enable_pagination">
                        <label for="enable_pagination">Enable pagination</label>
                    </div>
                    <div class="help-text">Fetch multiple pages to get more than 50 posts</div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="include_images" name="include_images">
                        <label for="include_images">Download images</label>
                    </div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="include_videos" name="include_videos">
                        <label for="include_videos">Download videos</label>
                    </div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="include_other_media" name="include_other_media">
                        <label for="include_other_media">Download other media</label>
                    </div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Run Mode</div>
                <div class="form-group">
                    <label for="run_mode">Mode</label>
                    <select id="run_mode" name="run_mode">
                        <option value="once">Once</option>
                        <option value="continuous">Continuous</option>
                    </select>
                </div>
                <div class="form-group" id="interval-group">
                    <label for="interval">Interval</label>
                    <input type="text" id="interval" name="interval" placeholder="5m">
                    <div class="help-text">For continuous mode (e.g., "5m", "1h", "30m")</div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Web Server</div>
                <div class="form-group">
                    <label for="web_host">Host</label>
                    <input type="text" id="web_host" name="web_host" placeholder="localhost">
                    <div class="help-text">Host to bind to (use 0.0.0.0 for external access)</div>
                </div>
                <div class="form-group">
                    <label for="web_port">Port</label>
                    <input type="number" id="web_port" name="web_port" min="1" max="65535">
                </div>
            </div>

            <div class="section">
                <div class="section-title">Thumbnails</div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="thumbnails_enabled" name="thumbnails_enabled">
                        <label for="thumbnails_enabled">Enable thumbnail generation</label>
                    </div>
                    <div class="help-text">Generate thumbnails for faster web UI loading</div>
                </div>
                <div class="form-group">
                    <label for="thumbnails_max_width">Max Width</label>
                    <input type="number" id="thumbnails_max_width" name="thumbnails_max_width" min="50" max="2000" placeholder="400">
                    <div class="help-text">Maximum thumbnail width (maintains aspect ratio)</div>
                </div>
                <div class="form-group">
                    <label for="thumbnails_max_height">Max Height</label>
                    <input type="number" id="thumbnails_max_height" name="thumbnails_max_height" min="50" max="2000" placeholder="400">
                    <div class="help-text">Maximum thumbnail height (maintains aspect ratio)</div>
                </div>
                <div class="form-group">
                    <label for="thumbnails_quality">JPEG Quality</label>
                    <input type="number" id="thumbnails_quality" name="thumbnails_quality" min="1" max="100" placeholder="85">
                    <div class="help-text">JPEG quality (1-100, higher is better)</div>
                </div>
                <div class="form-group">
                    <label for="thumbnails_directory">Directory</label>
                    <input type="text" id="thumbnails_directory" name="thumbnails_directory" placeholder="./thumbnails">
                    <div class="help-text">Where to store generated thumbnails</div>
                </div>
                <div class="form-group">
                    <label for="thumbnails_video_method">Video Thumbnail Method</label>
                    <input type="text" id="thumbnails_video_method" name="thumbnails_video_method" placeholder="ffmpeg">
                    <div class="help-text">Method for video thumbnails (requires ffmpeg)</div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Image Recognition & Auto-Tagging</div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="recognition_enabled" name="recognition_enabled">
                        <label for="recognition_enabled">Enable AI-powered image recognition</label>
                    </div>
                    <div class="help-text">Requires Ollama with a vision model (e.g., llama3.2-vision)</div>
                </div>
                <div class="form-group">
                    <label for="recognition_provider">Provider</label>
                    <input type="text" id="recognition_provider" name="recognition_provider" placeholder="ollama">
                    <div class="help-text">Recognition provider (currently only "ollama" supported)</div>
                </div>
                <div class="form-group">
                    <label for="recognition_ollama_url">Ollama URL</label>
                    <input type="text" id="recognition_ollama_url" name="recognition_ollama_url" placeholder="http://localhost:11434">
                    <div class="help-text">URL of your Ollama API instance</div>
                </div>
                <div class="form-group">
                    <label for="recognition_model">Model</label>
                    <input type="text" id="recognition_model" name="recognition_model" placeholder="llama3.2-vision:latest">
                    <div class="help-text">Ollama model to use (install with: ollama pull llama3.2-vision:latest)</div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="recognition_auto_tag" name="recognition_auto_tag">
                        <label for="recognition_auto_tag">Automatically create tags from AI classifications</label>
                    </div>
                </div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="recognition_nsfw_detection" name="recognition_nsfw_detection">
                        <label for="recognition_nsfw_detection">Enable NSFW content detection (experimental)</label>
                    </div>
                </div>
                <div class="form-group">
                    <label for="recognition_confidence_threshold">Confidence Threshold</label>
                    <input type="number" id="recognition_confidence_threshold" name="recognition_confidence_threshold" min="0" max="1" step="0.1" placeholder="0.6">
                    <div class="help-text">Minimum confidence (0.0-1.0) for auto-tagging</div>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Search</div>
                <div class="form-group">
                    <div class="checkbox-group">
                        <input type="checkbox" id="search_rebuild_index" name="search_rebuild_index">
                        <label for="search_rebuild_index">Rebuild search index on startup</label>
                    </div>
                    <div class="help-text">Only enable if search results seem incorrect (may slow startup)</div>
                </div>
            </div>

            <div class="alert alert-warning">
                <strong>Note:</strong> Changes will be saved to the config file. Some settings may require restarting the application to take effect.
            </div>

            <div class="form-actions">
                <button type="submit" class="btn">Save Configuration</button>
                <button type="button" class="btn btn-secondary" onclick="loadConfig()">Reset Form</button>
            </div>
        </form>
    </div>

    <script>
        let currentConfig = null;

        // Load configuration on page load
        document.addEventListener('DOMContentLoaded', () => {
            loadConfig();

            // Toggle interval field visibility based on run mode
            document.getElementById('run_mode').addEventListener('change', (e) => {
                const intervalGroup = document.getElementById('interval-group');
                intervalGroup.style.display = e.target.value === 'continuous' ? 'block' : 'none';
            });
        });

        function loadConfig() {
            showAlert('Loading configuration...', 'info');

            fetch('/api/config')
                .then(r => r.json())
                .then(config => {
                    currentConfig = config;
                    populateForm(config);
                    clearAlert();
                })
                .catch(err => {
                    showAlert('Failed to load configuration: ' + err.message, 'error');
                });
        }

        function populateForm(config) {
            document.getElementById('instance').value = config.lemmy.instance || '';
            document.getElementById('username').value = config.lemmy.username || '';
            document.getElementById('password').value = '';
            document.getElementById('communities').value = JSON.stringify(config.lemmy.communities || [], null, 2);

            document.getElementById('base_directory').value = config.storage.base_directory || '';
            document.getElementById('database_path').value = config.database.path || '';

            document.getElementById('max_posts_per_run').value = config.scraper.max_posts_per_run || 50;
            document.getElementById('sort_type').value = config.scraper.sort_type || 'Hot';
            document.getElementById('seen_posts_threshold').value = config.scraper.seen_posts_threshold || 5;
            document.getElementById('stop_at_seen_posts').checked = config.scraper.stop_at_seen_posts || false;
            document.getElementById('skip_seen_posts').checked = config.scraper.skip_seen_posts || false;
            document.getElementById('enable_pagination').checked = config.scraper.enable_pagination || false;
            document.getElementById('include_images').checked = config.scraper.include_images || false;
            document.getElementById('include_videos').checked = config.scraper.include_videos || false;
            document.getElementById('include_other_media').checked = config.scraper.include_other_media || false;

            document.getElementById('run_mode').value = config.run_mode.mode || 'once';
            const intervalNs = config.run_mode.interval || 0;
            const intervalStr = intervalNs ? formatDuration(intervalNs) : '5m';
            document.getElementById('interval').value = intervalStr;

            const intervalGroup = document.getElementById('interval-group');
            intervalGroup.style.display = config.run_mode.mode === 'continuous' ? 'block' : 'none';

            document.getElementById('web_host').value = config.web_server.host || 'localhost';
            document.getElementById('web_port').value = config.web_server.port || 8080;

            // Thumbnails
            const thumbnails = config.thumbnails || {};
            document.getElementById('thumbnails_enabled').checked = thumbnails.enabled || false;
            document.getElementById('thumbnails_max_width').value = thumbnails.max_width || 400;
            document.getElementById('thumbnails_max_height').value = thumbnails.max_height || 400;
            document.getElementById('thumbnails_quality').value = thumbnails.quality || 85;
            document.getElementById('thumbnails_directory').value = thumbnails.directory || './thumbnails';
            document.getElementById('thumbnails_video_method').value = thumbnails.video_method || 'ffmpeg';

            // Recognition
            const recognition = config.recognition || {};
            document.getElementById('recognition_enabled').checked = recognition.enabled || false;
            document.getElementById('recognition_provider').value = recognition.provider || 'ollama';
            document.getElementById('recognition_ollama_url').value = recognition.ollama_url || 'http://localhost:11434';
            document.getElementById('recognition_model').value = recognition.model || 'llama3.2-vision:latest';
            document.getElementById('recognition_auto_tag').checked = recognition.auto_tag || false;
            document.getElementById('recognition_nsfw_detection').checked = recognition.nsfw_detection || false;
            document.getElementById('recognition_confidence_threshold').value = recognition.confidence_threshold || 0.6;

            // Search
            const search = config.search || {};
            document.getElementById('search_rebuild_index').checked = search.rebuild_index || false;
        }

        function formatDuration(ns) {
            const minutes = Math.floor(ns / (60 * 1000000000));
            const hours = Math.floor(minutes / 60);

            if (hours > 0) {
                return hours + 'h';
            } else {
                return minutes + 'm';
            }
        }

        document.getElementById('config-form').addEventListener('submit', (e) => {
            e.preventDefault();
            saveConfig();
        });

        function saveConfig() {
            const formData = new FormData(document.getElementById('config-form'));

            try {
                const communities = JSON.parse(document.getElementById('communities').value || '[]');

                const config = {
                    lemmy: {
                        instance: formData.get('instance'),
                        username: formData.get('username'),
                        password: formData.get('password') || '',
                        communities: communities
                    },
                    storage: {
                        base_directory: formData.get('base_directory')
                    },
                    database: {
                        path: formData.get('database_path')
                    },
                    scraper: {
                        max_posts_per_run: parseInt(formData.get('max_posts_per_run')),
                        sort_type: formData.get('sort_type'),
                        seen_posts_threshold: parseInt(formData.get('seen_posts_threshold')),
                        stop_at_seen_posts: formData.get('stop_at_seen_posts') === 'on',
                        skip_seen_posts: formData.get('skip_seen_posts') === 'on',
                        enable_pagination: formData.get('enable_pagination') === 'on',
                        include_images: formData.get('include_images') === 'on',
                        include_videos: formData.get('include_videos') === 'on',
                        include_other_media: formData.get('include_other_media') === 'on'
                    },
                    run_mode: {
                        mode: formData.get('run_mode'),
                        interval: parseDuration(formData.get('interval'))
                    },
                    web_server: {
                        enabled: true,
                        host: formData.get('web_host'),
                        port: parseInt(formData.get('web_port'))
                    },
                    thumbnails: {
                        enabled: formData.get('thumbnails_enabled') === 'on',
                        max_width: parseInt(formData.get('thumbnails_max_width')) || 400,
                        max_height: parseInt(formData.get('thumbnails_max_height')) || 400,
                        quality: parseInt(formData.get('thumbnails_quality')) || 85,
                        directory: formData.get('thumbnails_directory') || './thumbnails',
                        video_method: formData.get('thumbnails_video_method') || 'ffmpeg'
                    },
                    recognition: {
                        enabled: formData.get('recognition_enabled') === 'on',
                        provider: formData.get('recognition_provider') || 'ollama',
                        ollama_url: formData.get('recognition_ollama_url') || 'http://localhost:11434',
                        model: formData.get('recognition_model') || 'llama3.2-vision:latest',
                        auto_tag: formData.get('recognition_auto_tag') === 'on',
                        nsfw_detection: formData.get('recognition_nsfw_detection') === 'on',
                        confidence_threshold: parseFloat(formData.get('recognition_confidence_threshold')) || 0.6
                    },
                    search: {
                        rebuild_index: formData.get('search_rebuild_index') === 'on'
                    }
                };

                showAlert('Saving configuration...', 'info');

                fetch('/api/config', {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(config)
                })
                .then(r => {
                    if (!r.ok) {
                        return r.text().then(text => { throw new Error(text); });
                    }
                    return r.json();
                })
                .then(data => {
                    showAlert(data.message || 'Configuration saved successfully!', 'success');
                    setTimeout(() => loadConfig(), 1000);
                })
                .catch(err => {
                    showAlert('Failed to save configuration: ' + err.message, 'error');
                });

            } catch (err) {
                showAlert('Invalid form data: ' + err.message, 'error');
            }
        }

        function parseDuration(str) {
            if (!str) return 0;

            const match = str.match(/^(\d+)([smh])$/);
            if (!match) return 0;

            const value = parseInt(match[1]);
            const unit = match[2];

            switch (unit) {
                case 's': return value * 1000000000;
                case 'm': return value * 60 * 1000000000;
                case 'h': return value * 3600 * 1000000000;
                default: return 0;
            }
        }

        function showAlert(message, type) {
            const container = document.getElementById('alert-container');
            const alertClass = type === 'error' ? 'alert-error' :
                              type === 'success' ? 'alert-success' :
                              'alert-warning';

            container.innerHTML = '<div class="alert ' + alertClass + '">' + escapeHtml(message) + '</div>';
            container.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        }

        function clearAlert() {
            document.getElementById('alert-container').innerHTML = '';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>
{{end}}`

const statsTemplate = `{{define "stats"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Statistics - Lemmy Media</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f0f0f; color: #e0e0e0; }
        .header { background: #1a1a1a; border-bottom: 1px solid #2a2a2a; padding: 16px; }
        .header-content { max-width: 1200px; margin: 0 auto; display: flex; justify-content: space-between; align-items: center; }
        .header h1 { font-size: 24px; }
        .header a { color: #4a9eff; text-decoration: none; }
        .container { max-width: 1200px; margin: 0 auto; padding: 24px; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; margin-bottom: 32px; }
        .stat-card { background: #1a1a1a; padding: 20px; border-radius: 8px; }
        .stat-card h3 { color: #999; font-size: 14px; margin-bottom: 8px; }
        .stat-card .value { font-size: 32px; font-weight: 600; color: #4a9eff; }
        table { width: 100%; background: #1a1a1a; border-radius: 8px; overflow: hidden; margin-bottom: 24px; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #2a2a2a; }
        th { background: #252525; color: #999; font-weight: 500; }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>📊 Statistics</h1>
            <a href="/">← Back to Media</a>
        </div>
    </div>
    <div class="container">
        <div class="stats-grid">
            {{if .Stats.total_media}}
            <div class="stat-card">
                <h3>Total Media</h3>
                <div class="value">{{.Stats.total_media}}</div>
            </div>
            {{end}}
        </div>

        {{if .TopCreators}}
        <h2 style="margin-bottom: 16px;">Top Creators</h2>
        <table>
            <thead>
                <tr>
                    <th>Creator</th>
                    <th>Media Count</th>
                    <th>Total Score</th>
                </tr>
            </thead>
            <tbody>
                {{range .TopCreators}}
                <tr>
                    <td>{{index . "author_name"}}</td>
                    <td>{{index . "media_count"}}</td>
                    <td>{{index . "total_score"}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}

        {{if .Timeline}}
        <h2 style="margin-bottom: 16px;">Recent Downloads (Last 30 Days)</h2>
        <table>
            <thead>
                <tr>
                    <th>Period</th>
                    <th>Count</th>
                </tr>
            </thead>
            <tbody>
                {{range .Timeline}}
                <tr>
                    <td>{{index . "period"}}</td>
                    <td>{{index . "count"}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
    </div>
</body>
</html>
{{end}}`

const tagsTemplate = `{{define "tags"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tags - Lemmy Media</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f0f0f; color: #e0e0e0; }
        .header { background: #1a1a1a; border-bottom: 1px solid #2a2a2a; padding: 16px; }
        .header-content { max-width: 1200px; margin: 0 auto; display: flex; justify-content: space-between; align-items: center; }
        .header h1 { font-size: 24px; }
        .header a { color: #4a9eff; text-decoration: none; }
        .container { max-width: 1200px; margin: 0 auto; padding: 24px; }
        .tag-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; }
        .tag-card { background: #1a1a1a; padding: 16px; border-radius: 8px; border-left: 4px solid; }
        .tag-card h3 { font-size: 16px; margin-bottom: 4px; }
        .tag-card .meta { font-size: 12px; color: #999; }
        .btn { display: inline-block; background: #4a9eff; color: #fff; padding: 8px 16px; border-radius: 4px; text-decoration: none; margin-bottom: 24px; border: none; cursor: pointer; }
        .btn:hover { background: #3a8eef; }
        .btn-secondary { background: #2a2a2a; border: 1px solid #3a3a3a; }
        .btn-secondary:hover { background: #333; }
        .button-group { display: flex; gap: 12px; margin-bottom: 24px; }
        #backfill-status { margin-top: 12px; padding: 12px; background: #1a3a1a; border: 1px solid #2a5a2a; color: #6fd46f; border-radius: 4px; display: none; }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>🏷️ Tags</h1>
            <a href="/">← Back to Media</a>
        </div>
    </div>
    <div class="container">
        <div class="button-group">
            <a href="#" class="btn" onclick="alert('Use API to create tags: POST /api/tags'); return false;">+ Create Tag</a>
            <button class="btn btn-secondary" onclick="runBackfill()">🤖 Auto-Tag Untagged Media</button>
        </div>
        <div id="backfill-status"></div>
        <div class="tag-grid">
            {{range .Tags}}
            <div class="tag-card" style="border-left-color: {{index . "color"}};">
                <h3>{{index . "name"}}</h3>
                <div class="meta">
                    {{if index . "auto_generated"}}Auto-generated{{else}}User-created{{end}}
                </div>
            </div>
            {{else}}
            <p style="color: #999;">No tags yet. Tags can be created via the API or auto-generated from image recognition.</p>
            {{end}}
        </div>
    </div>
    <script>
        function runBackfill() {
            if (!confirm('This will auto-tag all media that currently has no tags using AI image recognition. This may take a while depending on the number of untagged images. Continue?')) {
                return;
            }

            const statusDiv = document.getElementById('backfill-status');
            statusDiv.style.display = 'block';
            statusDiv.innerHTML = '⏳ Starting auto-tag backfill... Check server logs for detailed progress.';

            fetch('/api/tags/backfill', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                }
            })
            .then(response => {
                if (!response.ok) {
                    return response.text().then(text => {
                        throw new Error(text || 'Backfill request failed');
                    });
                }
                return response.json();
            })
            .then(data => {
                statusDiv.innerHTML = '✓ ' + (data.message || 'Auto-tag backfill started! Check server logs for progress.');
                setTimeout(() => {
                    statusDiv.style.display = 'none';
                    location.reload(); // Reload to show newly created tags
                }, 3000);
            })
            .catch(error => {
                statusDiv.style.background = '#3a1a1a';
                statusDiv.style.borderColor = '#5a2a2a';
                statusDiv.style.color = '#f46f6f';
                statusDiv.innerHTML = '✗ Error: ' + error.message;
            });
        }
    </script>
</body>
</html>
{{end}}`
