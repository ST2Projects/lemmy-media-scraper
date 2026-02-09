package thumbnails

import (
	"fmt"
	"os"

	"github.com/ST2Projects/lemmy-media-scraper/internal/database"
	log "github.com/sirupsen/logrus"
)

// BackfillThumbnails generates thumbnails for all media items that don't have them yet.
// It queries the database for media without thumbnails and generates them one by one.
// Errors on individual items are logged but do not stop the backfill.
func BackfillThumbnails(gen *Generator, db *database.DB) {
	if gen == nil {
		return
	}

	media, err := db.GetMediaWithoutThumbnails()
	if err != nil {
		log.Errorf("Failed to query media without thumbnails: %v", err)
		return
	}

	if len(media) == 0 {
		log.Debug("No media items need thumbnail generation")
		return
	}

	log.Infof("Generating thumbnails for %d media items", len(media))

	generated := 0
	skipped := 0
	errors := 0

	for _, item := range media {
		// Verify the source file exists before attempting thumbnail generation
		if _, err := os.Stat(item.FilePath); os.IsNotExist(err) {
			log.Debugf("Source file missing for media %d: %s", item.ID, item.FilePath)
			skipped++
			continue
		}

		// Map the stored media type to the format the generator expects
		mediaType := mapMediaType(item.MediaType)
		if mediaType == "" {
			log.Debugf("Unsupported media type for thumbnails: %s (media %d)", item.MediaType, item.ID)
			skipped++
			continue
		}

		thumbnailPath, width, height, err := gen.GenerateThumbnail(item.FilePath, mediaType)
		if err != nil {
			log.Debugf("Failed to generate thumbnail for media %d: %v", item.ID, err)
			errors++
			continue
		}

		if err := db.SaveThumbnail(item.ID, thumbnailPath, width, height); err != nil {
			log.Errorf("Failed to save thumbnail record for media %d: %v", item.ID, err)
			errors++
			continue
		}

		generated++
		if generated%100 == 0 {
			log.Infof("Thumbnail backfill progress: %d/%d generated", generated, len(media))
		}
	}

	log.Infof("Thumbnail backfill complete: %d generated, %d skipped, %d errors (out of %d)",
		generated, skipped, errors, len(media))
}

// mapMediaType converts the stored media type ("image", "video") to a MIME-type prefix
// that the thumbnail generator expects ("image/jpeg", "video/mp4").
func mapMediaType(mediaType string) string {
	switch mediaType {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	default:
		return ""
	}
}

// GenerateForMedia generates a thumbnail for a single media item and saves it to the database.
// Returns the thumbnail path or an error. This is used by the scraper for newly downloaded media.
func GenerateForMedia(gen *Generator, db *database.DB, mediaID int64, filePath string, mediaType string) error {
	if gen == nil {
		return nil
	}

	mappedType := mapMediaType(mediaType)
	if mappedType == "" {
		return fmt.Errorf("unsupported media type for thumbnails: %s", mediaType)
	}

	thumbnailPath, width, height, err := gen.GenerateThumbnail(filePath, mappedType)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	if err := db.SaveThumbnail(mediaID, thumbnailPath, width, height); err != nil {
		return fmt.Errorf("failed to save thumbnail: %w", err)
	}

	return nil
}
