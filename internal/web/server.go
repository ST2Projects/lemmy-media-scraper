package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ST2Projects/lemmy-media-scraper/internal/config"
	"github.com/ST2Projects/lemmy-media-scraper/internal/database"
	"github.com/ST2Projects/lemmy-media-scraper/internal/progress"
	"github.com/ST2Projects/lemmy-media-scraper/internal/thumbnails"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Server represents the web server
type Server struct {
	Config            *config.Config
	ConfigPath        string
	DB                *database.DB
	ProgressTracker   *progress.Tracker
	ThumbnailGen      *thumbnails.Generator
	handler           http.Handler
	websocketUpgrader websocket.Upgrader
}

// New creates a new web server
func New(cfg *config.Config, configPath string, db *database.DB, progressTracker *progress.Tracker, thumbnailGen *thumbnails.Generator) *Server {
	s := &Server{
		Config:          cfg,
		ConfigPath:      configPath,
		DB:              db,
		ProgressTracker: progressTracker,
		ThumbnailGen:    thumbnailGen,
		websocketUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins - WebSocket provides read-only progress data
				// and the SvelteKit frontend handles authentication
				return true
			},
		},
	}
	s.setupRoutes()
	return s
}

// corsMiddleware adds CORS headers for frontend requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow any origin - the Go API is accessed by the SvelteKit frontend
		// (server-side proxy) and by browsers for WebSocket connections.
		// Authentication is handled by the SvelteKit layer.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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

		// Skip restrictive CSP for media/thumbnail endpoints (they serve files)
		if !strings.HasPrefix(r.URL.Path, "/media/") && !strings.HasPrefix(r.URL.Path, "/thumbnails/") {
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// API routes
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

	// Advanced statistics endpoints
	mux.HandleFunc("/api/stats/timeline", s.handleStatsTimeline)
	mux.HandleFunc("/api/stats/top-creators", s.handleStatsTopCreators)
	mux.HandleFunc("/api/stats/storage", s.handleStatsStorage)

	// WebSocket endpoint for real-time progress
	mux.HandleFunc("/ws/progress", s.handleWebSocket)

	// Serve media files and thumbnails
	mux.HandleFunc("/media/", s.handleServeMedia)
	mux.HandleFunc("/thumbnails/", s.handleServeThumbnail)

	// Wrap with middleware: CORS first, then security headers
	s.handler = corsMiddleware(securityHeadersMiddleware(mux))
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.WebServer.Host, s.Config.WebServer.Port)
	log.Infof("Starting API server on http://%s", addr)
	return http.ListenAndServe(addr, s.handler)
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
