package tags

import (
	"fmt"
	"strings"

	"github.com/ST2Projects/lemmy-media-scraper/internal/database"
	"github.com/ST2Projects/lemmy-media-scraper/internal/recognition"
	log "github.com/sirupsen/logrus"
)

// Manager handles tag operations and auto-tagging
type Manager struct {
	DB         *database.DB
	Classifier recognition.Classifier
	AutoTag    bool
}

// NewManager creates a new tag manager
func NewManager(db *database.DB, classifier recognition.Classifier, autoTag bool) *Manager {
	return &Manager{
		DB:         db,
		Classifier: classifier,
		AutoTag:    autoTag,
	}
}

// AutoTagMedia automatically generates and assigns tags based on image recognition
func (m *Manager) AutoTagMedia(mediaID int64, imagePath string) error {
	if !m.AutoTag || m.Classifier == nil {
		return nil
	}

	log.Infof("Auto-tagging media ID %d: %s", mediaID, imagePath)

	// Classify the image
	classification, err := m.Classifier.Classify(imagePath)
	if err != nil {
		log.Errorf("Failed to classify image %s (media ID %d): %v", imagePath, mediaID, err)
		return fmt.Errorf("failed to classify image: %w", err)
	}

	// Combine labels and categories
	allTags := append(classification.Labels, classification.Categories...)
	allTags = uniqueStrings(allTags)

	if len(allTags) == 0 {
		log.Warnf("No tags generated for media ID %d (confidence threshold may be too high)", mediaID)
		return nil
	}

	log.Infof("AI classification returned %d potential tags for media ID %d: %v", len(allTags), mediaID, allTags)

	// Create and assign tags
	assignedCount := 0
	createdCount := 0
	for _, tagName := range allTags {
		if tagName == "" || len(tagName) < 2 {
			continue
		}

		// Normalize tag name
		originalName := tagName
		tagName = normalizeTagName(tagName)

		// Get or create tag
		tag, err := m.DB.GetTagByName(tagName)
		if err != nil {
			log.Warnf("Error getting tag '%s': %v", tagName, err)
			continue
		}

		var tagID int64
		if tag == nil {
			// Create new auto-generated tag
			tagID, err = m.DB.CreateTag(tagName, generateColor(tagName), true)
			if err != nil {
				log.Warnf("Failed to create tag '%s': %v", tagName, err)
				continue
			}
			log.Infof("Created auto-generated tag: %s (from: %s)", tagName, originalName)
			createdCount++
		} else {
			tagID = tag["id"].(int64)
		}

		// Assign tag to media
		if err := m.DB.AssignTagToMedia(mediaID, tagID); err != nil {
			// Check if it's a duplicate assignment (not really an error)
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				log.Debugf("Tag '%s' already assigned to media %d", tagName, mediaID)
			} else {
				log.Warnf("Failed to assign tag '%s' to media %d: %v", tagName, mediaID, err)
			}
			continue
		}
		assignedCount++
	}

	if assignedCount > 0 {
		log.Infof("Successfully auto-tagged media ID %d with %d tags (%d new tags created)", mediaID, assignedCount, createdCount)
	} else {
		log.Warnf("No tags were assigned to media ID %d (all tags may already exist or failed to create)", mediaID)
	}

	return nil
}

// CreateUserTag creates a manually-created tag
func (m *Manager) CreateUserTag(name string, color string) (int64, error) {
	name = normalizeTagName(name)

	// Check if tag already exists
	existing, err := m.DB.GetTagByName(name)
	if err != nil {
		return 0, fmt.Errorf("failed to check existing tag: %w", err)
	}

	if existing != nil {
		return existing["id"].(int64), nil
	}

	// Create new user tag
	if color == "" {
		color = generateColor(name)
	}

	return m.DB.CreateTag(name, color, false)
}

// AssignTag assigns an existing tag to media
func (m *Manager) AssignTag(mediaID int64, tagID int64) error {
	return m.DB.AssignTagToMedia(mediaID, tagID)
}

// RemoveTag removes a tag from media
func (m *Manager) RemoveTag(mediaID int64, tagID int64) error {
	return m.DB.RemoveTagFromMedia(mediaID, tagID)
}

// GetTagsForMedia retrieves all tags for a media item
func (m *Manager) GetTagsForMedia(mediaID int64) ([]map[string]interface{}, error) {
	return m.DB.GetTagsForMedia(mediaID)
}

// GetAllTags retrieves all tags
func (m *Manager) GetAllTags() ([]map[string]interface{}, error) {
	return m.DB.GetAllTags()
}

// DeleteTag deletes a tag
func (m *Manager) DeleteTag(tagID int64) error {
	return m.DB.DeleteTag(tagID)
}

// BackfillUntaggedMedia auto-tags all media that currently has no tags
func (m *Manager) BackfillUntaggedMedia() (int, int, error) {
	if !m.AutoTag || m.Classifier == nil {
		return 0, 0, fmt.Errorf("auto-tagging is not enabled")
	}

	log.Info("Starting auto-tag backfill for untagged media...")

	// Get all untagged images
	untagged, err := m.DB.GetUntaggedImages()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get untagged images: %w", err)
	}

	total := len(untagged)
	if total == 0 {
		log.Info("No untagged media found")
		return 0, 0, nil
	}

	log.Infof("Found %d untagged images to process", total)

	successCount := 0
	errorCount := 0

	for i, media := range untagged {
		mediaID, ok := media["id"].(int64)
		if !ok {
			log.Warnf("Invalid media ID type for item %d", i)
			errorCount++
			continue
		}

		filePath, ok := media["file_path"].(string)
		if !ok {
			log.Warnf("Invalid file path for media ID %d", mediaID)
			errorCount++
			continue
		}

		log.Infof("Processing %d/%d: Media ID %d", i+1, total, mediaID)

		if err := m.AutoTagMedia(mediaID, filePath); err != nil {
			log.Errorf("Failed to auto-tag media ID %d: %v", mediaID, err)
			errorCount++
		} else {
			successCount++
		}
	}

	log.Infof("Backfill complete: %d succeeded, %d failed out of %d total", successCount, errorCount, total)

	return successCount, errorCount, nil
}

// Helper functions

// normalizeTagName normalizes a tag name
func normalizeTagName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Trim whitespace
	name = strings.TrimSpace(name)

	// Replace spaces and special characters
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove invalid characters
	var cleaned strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
}

// generateColor generates a color for a tag based on its name
func generateColor(name string) string {
	// Simple hash-based color generation
	colors := []string{
		"#3B82F6", // blue
		"#10B981", // green
		"#F59E0B", // yellow
		"#EF4444", // red
		"#8B5CF6", // purple
		"#EC4899", // pink
		"#06B6D4", // cyan
		"#F97316", // orange
		"#14B8A6", // teal
		"#6366F1", // indigo
	}

	// Hash the name to get a color index
	hash := 0
	for _, r := range name {
		hash = int(r) + ((hash << 5) - hash)
	}

	if hash < 0 {
		hash = -hash
	}

	return colors[hash%len(colors)]
}

// uniqueStrings returns a slice with unique strings
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, val := range slice {
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}

	return result
}
