package web

import (
	"encoding/json"
	"net/http"
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

// handleTags handles GET (list all tags) and POST (create tag) requests
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tags, err := s.TagManager.GetAllTags()
		if err != nil {
			log.Errorf("Failed to get tags: %v", err)
			http.Error(w, "Failed to retrieve tags", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{"tags": tags})

	case http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "Tag name is required", http.StatusBadRequest)
			return
		}

		tagID, err := s.TagManager.CreateUserTag(req.Name, req.Color)
		if err != nil {
			log.Errorf("Failed to create tag: %v", err)
			http.Error(w, "Failed to create tag", http.StatusInternalServerError)
			return
		}

		tag, _ := s.DB.GetTagByID(tagID)
		respondJSON(w, map[string]interface{}{"tag": tag})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTagByID handles GET and DELETE for a specific tag
func (s *Server) handleTagByID(w http.ResponseWriter, r *http.Request) {
	// Extract tag ID from path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}

	tagID, err := strconv.ParseInt(pathParts[2], 10, 64)
	if err != nil {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tag, err := s.DB.GetTagByID(tagID)
		if err != nil {
			http.Error(w, "Tag not found", http.StatusNotFound)
			return
		}
		respondJSON(w, map[string]interface{}{"tag": tag})

	case http.MethodDelete:
		if err := s.TagManager.DeleteTag(tagID); err != nil {
			log.Errorf("Failed to delete tag: %v", err)
			http.Error(w, "Failed to delete tag", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMediaTags handles GET (list tags for media), POST (assign tag), and DELETE (remove tag)
func (s *Server) handleMediaTags(w http.ResponseWriter, r *http.Request) {
	// Extract media ID from path: /api/media-tags/{mediaID}[/{tagID}]
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	mediaID, err := strconv.ParseInt(pathParts[2], 10, 64)
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tags, err := s.TagManager.GetTagsForMedia(mediaID)
		if err != nil {
			log.Errorf("Failed to get media tags: %v", err)
			http.Error(w, "Failed to retrieve tags", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{"tags": tags})

	case http.MethodPost:
		var req struct {
			TagID int64 `json:"tag_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.TagManager.AssignTag(mediaID, req.TagID); err != nil {
			log.Errorf("Failed to assign tag: %v", err)
			http.Error(w, "Failed to assign tag", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{"success": true})

	case http.MethodDelete:
		// Extract tag ID from path
		if len(pathParts) < 4 {
			http.Error(w, "Invalid tag ID", http.StatusBadRequest)
			return
		}

		tagID, err := strconv.ParseInt(pathParts[3], 10, 64)
		if err != nil {
			http.Error(w, "Invalid tag ID", http.StatusBadRequest)
			return
		}

		if err := s.TagManager.RemoveTag(mediaID, tagID); err != nil {
			log.Errorf("Failed to remove tag: %v", err)
			http.Error(w, "Failed to remove tag", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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

	// Serve the file
	http.ServeFile(w, r, thumbnailPath)
}

// respondJSON is a helper function to send JSON responses
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
