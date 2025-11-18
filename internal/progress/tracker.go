package progress

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Status represents the current scraper status
type Status struct {
	IsRunning        bool      `json:"is_running"`
	CurrentCommunity string    `json:"current_community"`
	PostsProcessed   int       `json:"posts_processed"`
	MediaDownloaded  int       `json:"media_downloaded"`
	ErrorsCount      int       `json:"errors_count"`
	CurrentOperation string    `json:"current_operation"`
	Progress         float64   `json:"progress"` // 0-100
	StartedAt        time.Time `json:"started_at"`
	ETA              string    `json:"eta,omitempty"`
}

// Tracker manages real-time progress updates
type Tracker struct {
	mu        sync.RWMutex
	status    Status
	clients   map[*websocket.Conn]bool
	broadcast chan Status
	register  chan *websocket.Conn
	unregister chan *websocket.Conn
}

// NewTracker creates a new progress tracker
func NewTracker() *Tracker {
	tracker := &Tracker{
		status:     Status{IsRunning: false},
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan Status, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}

	go tracker.run()

	return tracker
}

// run handles WebSocket connections and broadcasts
func (t *Tracker) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-t.register:
			t.mu.Lock()
			t.clients[client] = true
			t.mu.Unlock()

			// Send current status to new client
			t.sendToClient(client, t.GetStatus())

		case client := <-t.unregister:
			t.mu.Lock()
			if _, ok := t.clients[client]; ok {
				delete(t.clients, client)
				client.Close()
			}
			t.mu.Unlock()

		case status := <-t.broadcast:
			t.mu.RLock()
			for client := range t.clients {
				t.sendToClient(client, status)
			}
			t.mu.RUnlock()

		case <-ticker.C:
			// Periodic status update
			t.mu.RLock()
			status := t.status
			t.mu.RUnlock()

			if status.IsRunning {
				// Update ETA if running
				t.updateETA()
			}
		}
	}
}

// sendToClient sends status to a single client
func (t *Tracker) sendToClient(client *websocket.Conn, status Status) {
	data, err := json.Marshal(status)
	if err != nil {
		log.Errorf("Failed to marshal status: %v", err)
		return
	}

	client.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Debugf("Failed to send to client: %v", err)
		// Client will be unregistered on next read error
	}
}

// RegisterClient registers a new WebSocket client
func (t *Tracker) RegisterClient(client *websocket.Conn) {
	t.register <- client
}

// UnregisterClient unregisters a WebSocket client
func (t *Tracker) UnregisterClient(client *websocket.Conn) {
	t.unregister <- client
}

// Start marks the beginning of a scrape operation
func (t *Tracker) Start() {
	t.mu.Lock()
	t.status = Status{
		IsRunning:      true,
		StartedAt:      time.Now(),
		PostsProcessed: 0,
		MediaDownloaded: 0,
		ErrorsCount:    0,
		Progress:       0,
	}
	t.mu.Unlock()

	t.broadcastStatus()
}

// Stop marks the end of a scrape operation
func (t *Tracker) Stop() {
	t.mu.Lock()
	t.status.IsRunning = false
	t.status.CurrentOperation = "Completed"
	t.status.Progress = 100
	t.mu.Unlock()

	t.broadcastStatus()
}

// UpdateCommunity updates the current community being scraped
func (t *Tracker) UpdateCommunity(community string) {
	t.mu.Lock()
	t.status.CurrentCommunity = community
	t.status.CurrentOperation = "Scraping " + community
	t.mu.Unlock()

	t.broadcastStatus()
}

// IncrementPosts increments the posts processed counter
func (t *Tracker) IncrementPosts() {
	t.mu.Lock()
	t.status.PostsProcessed++
	t.mu.Unlock()

	t.broadcastStatus()
}

// IncrementMedia increments the media downloaded counter
func (t *Tracker) IncrementMedia() {
	t.mu.Lock()
	t.status.MediaDownloaded++
	t.mu.Unlock()

	t.broadcastStatus()
}

// IncrementErrors increments the errors counter
func (t *Tracker) IncrementErrors() {
	t.mu.Lock()
	t.status.ErrorsCount++
	t.mu.Unlock()

	t.broadcastStatus()
}

// UpdateOperation updates the current operation description
func (t *Tracker) UpdateOperation(operation string) {
	t.mu.Lock()
	t.status.CurrentOperation = operation
	t.mu.Unlock()

	t.broadcastStatus()
}

// UpdateProgress updates the progress percentage
func (t *Tracker) UpdateProgress(progress float64) {
	t.mu.Lock()
	t.status.Progress = progress
	t.mu.Unlock()

	t.broadcastStatus()
}

// GetStatus returns the current status
func (t *Tracker) GetStatus() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// broadcastStatus sends the current status to all connected clients
func (t *Tracker) broadcastStatus() {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()

	// Non-blocking send
	select {
	case t.broadcast <- status:
	default:
		// Channel full, skip this update
	}
}

// updateETA calculates and updates the estimated time to completion
func (t *Tracker) updateETA() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.status.IsRunning || t.status.PostsProcessed == 0 {
		return
	}

	elapsed := time.Since(t.status.StartedAt)
	avgTimePerPost := elapsed / time.Duration(t.status.PostsProcessed)

	// Estimate remaining posts (rough approximation)
	estimatedTotal := t.status.PostsProcessed * 2 // Assume we're halfway
	if t.status.Progress > 0 {
		estimatedTotal = int(float64(t.status.PostsProcessed) / (t.status.Progress / 100.0))
	}

	remaining := estimatedTotal - t.status.PostsProcessed
	if remaining < 0 {
		remaining = 0
	}

	eta := avgTimePerPost * time.Duration(remaining)

	// Format ETA
	if eta < time.Minute {
		t.status.ETA = "< 1 minute"
	} else if eta < time.Hour {
		minutes := int(eta.Minutes())
		t.status.ETA = fmt.Sprintf("%d minutes", minutes)
	} else {
		hours := int(eta.Hours())
		minutes := int(eta.Minutes()) % 60
		t.status.ETA = fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// GetClientCount returns the number of connected WebSocket clients
func (t *Tracker) GetClientCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.clients)
}
