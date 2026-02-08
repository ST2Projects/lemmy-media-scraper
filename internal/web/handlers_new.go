package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// handleSearch performs full-text search across media
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondJSON(w, map[string]interface{}{
			"results": []interface{}{},
			"total":   0,
		})
		return
	}

	if len(query) > 500 {
		http.Error(w, "Query too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	results, total, err := s.DB.SearchMedia(query, limit, offset)
	if err != nil {
		log.Errorf("Search error: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"results": results,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// handleStatsTimeline returns download statistics over time
func (s *Server) handleStatsTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day"
	}

	stats, err := s.DB.GetTimelineStats(period)
	if err != nil {
		log.Errorf("Failed to get timeline stats: %v", err)
		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"period": period,
		"data":   stats,
	})
}

// handleStatsTopCreators returns top content creators
func (s *Server) handleStatsTopCreators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	creators, err := s.DB.GetTopCreators(limit)
	if err != nil {
		log.Errorf("Failed to get top creators: %v", err)
		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"creators": creators,
	})
}

// handleStatsStorage returns storage breakdown by community and type
func (s *Server) handleStatsStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	breakdown, err := s.DB.GetStorageBreakdown()
	if err != nil {
		log.Errorf("Failed to get storage breakdown: %v", err)
		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}

	respondJSON(w, breakdown)
}

// handleWebSocket handles WebSocket connections for real-time progress updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.ProgressTracker == nil {
		http.Error(w, "Progress tracking not available", http.StatusServiceUnavailable)
		return
	}

	conn, err := s.websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("WebSocket upgrade error: %v", err)
		return
	}

	// Register client
	s.ProgressTracker.RegisterClient(conn)

	// Keep connection alive and listen for close
	go func() {
		defer s.ProgressTracker.UnregisterClient(conn)

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				// Connection closed
				break
			}
		}
	}()
}

// handleServeThumbnail serves thumbnail images
func (s *Server) handleServeThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract media ID from path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	mediaID, err := strconv.ParseInt(pathParts[1], 10, 64)
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	// Get thumbnail path from database
	thumbnailPath, err := s.DB.GetThumbnailPath(mediaID)
	if err != nil {
		log.Errorf("Failed to get thumbnail path: %v", err)
		http.Error(w, "Thumbnail not found", http.StatusNotFound)
		return
	}

	if thumbnailPath == "" {
		http.Error(w, "Thumbnail not found", http.StatusNotFound)
		return
	}

	// Prevent path traversal - clean and validate the thumbnail path
	cleanedPath := filepath.Clean(thumbnailPath)
	baseDir := filepath.Clean(s.Config.Thumbnails.Directory)

	// Resolve symlinks to prevent symlink-based bypasses
	resolvedPath, err := filepath.EvalSymlinks(cleanedPath)
	if err != nil {
		if _, statErr := os.Stat(cleanedPath); statErr != nil {
			http.Error(w, "Thumbnail not found", http.StatusNotFound)
			return
		}
		resolvedPath = cleanedPath
	}

	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		resolvedBase = baseDir
	}

	// Ensure the resolved path is within the thumbnails directory
	if !strings.HasPrefix(resolvedPath, resolvedBase) {
		log.Warnf("Blocked thumbnail path traversal attempt: %s -> %s", thumbnailPath, resolvedPath)
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	// Serve the file
	http.ServeFile(w, r, resolvedPath)
}

// respondJSON is a helper function to send JSON responses
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

