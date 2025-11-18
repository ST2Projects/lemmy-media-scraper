package thumbnails

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
)

// Generator handles thumbnail generation for images and videos
type Generator struct {
	MaxWidth     int
	MaxHeight    int
	Quality      int
	BaseDir      string
	VideoMethod  string
	FFmpegPath   string
}

// NewGenerator creates a new thumbnail generator
func NewGenerator(maxWidth, maxHeight, quality int, baseDir string, videoMethod string) *Generator {
	// Find ffmpeg path
	ffmpegPath, _ := exec.LookPath("ffmpeg")

	return &Generator{
		MaxWidth:    maxWidth,
		MaxHeight:   maxHeight,
		Quality:     quality,
		BaseDir:     baseDir,
		VideoMethod: videoMethod,
		FFmpegPath:  ffmpegPath,
	}
}

// GenerateThumbnail creates a thumbnail for the given media file
func (g *Generator) GenerateThumbnail(mediaPath string, mediaType string) (string, int, int, error) {
	// Ensure thumbnail directory exists
	if err := os.MkdirAll(g.BaseDir, 0755); err != nil {
		return "", 0, 0, fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Generate thumbnail path
	baseName := filepath.Base(mediaPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	thumbnailPath := filepath.Join(g.BaseDir, nameWithoutExt+".jpg")

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbnailPath); err == nil {
		// Thumbnail exists, get dimensions and return
		img, err := imaging.Open(thumbnailPath)
		if err == nil {
			bounds := img.Bounds()
			return thumbnailPath, bounds.Dx(), bounds.Dy(), nil
		}
	}

	var width, height int
	var err error

	// Generate based on media type
	if strings.HasPrefix(mediaType, "video/") {
		width, height, err = g.generateVideoThumbnail(mediaPath, thumbnailPath)
	} else if strings.HasPrefix(mediaType, "image/") {
		width, height, err = g.generateImageThumbnail(mediaPath, thumbnailPath)
	} else {
		return "", 0, 0, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	if err != nil {
		return "", 0, 0, err
	}

	return thumbnailPath, width, height, nil
}

// generateImageThumbnail creates a thumbnail from an image file
func (g *Generator) generateImageThumbnail(imagePath string, thumbnailPath string) (int, int, error) {
	// Open the image
	img, err := imaging.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Calculate thumbnail dimensions while maintaining aspect ratio
	thumbnail := imaging.Fit(img, g.MaxWidth, g.MaxHeight, imaging.Lanczos)

	// Save as JPEG
	err = imaging.Save(thumbnail, thumbnailPath, imaging.JPEGQuality(g.Quality))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to save thumbnail: %w", err)
	}

	// Get final dimensions
	finalBounds := thumbnail.Bounds()
	width := finalBounds.Dx()
	height := finalBounds.Dy()

	log.Debugf("Generated image thumbnail: %dx%d (original: %dx%d)", width, height, origWidth, origHeight)

	return width, height, nil
}

// generateVideoThumbnail creates a thumbnail from a video file using ffmpeg
func (g *Generator) generateVideoThumbnail(videoPath string, thumbnailPath string) (int, int, error) {
	if g.FFmpegPath == "" {
		return 0, 0, fmt.Errorf("ffmpeg not found, cannot generate video thumbnail")
	}

	// Use ffmpeg to extract a frame from the middle of the video
	// -ss 00:00:01 seeks to 1 second
	// -i input file
	// -vframes 1 extracts one frame
	// -vf scale applies filter to resize
	cmd := exec.Command(
		g.FFmpegPath,
		"-ss", "00:00:01", // Seek to 1 second
		"-i", videoPath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':min'(%d,ih)':force_original_aspect_ratio=decrease", g.MaxWidth, g.MaxHeight),
		"-q:v", "2", // Quality (2 is high quality)
		"-y", // Overwrite output file
		thumbnailPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	// Get dimensions of generated thumbnail
	img, err := imaging.Open(thumbnailPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open generated thumbnail: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	log.Debugf("Generated video thumbnail: %dx%d", width, height)

	return width, height, nil
}

// GetThumbnailPath returns the expected path for a thumbnail
func (g *Generator) GetThumbnailPath(mediaPath string) string {
	baseName := filepath.Base(mediaPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	return filepath.Join(g.BaseDir, nameWithoutExt+".jpg")
}

// ThumbnailExists checks if a thumbnail already exists
func (g *Generator) ThumbnailExists(mediaPath string) bool {
	thumbnailPath := g.GetThumbnailPath(mediaPath)
	_, err := os.Stat(thumbnailPath)
	return err == nil
}

// GenerateThumbnailFromBytes generates a thumbnail from image bytes in memory
func (g *Generator) GenerateThumbnailFromBytes(imageData []byte, outputPath string) (int, int, error) {
	// Decode image
	img, format, err := image.Decode(strings.NewReader(string(imageData)))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	// Generate thumbnail
	thumbnail := imaging.Fit(img, g.MaxWidth, g.MaxHeight, imaging.Lanczos)

	// Save based on format
	file, err := os.Create(outputPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	switch format {
	case "png":
		err = png.Encode(file, thumbnail)
	case "jpeg", "jpg":
		err = jpeg.Encode(file, thumbnail, &jpeg.Options{Quality: g.Quality})
	default:
		// Default to JPEG
		err = jpeg.Encode(file, thumbnail, &jpeg.Options{Quality: g.Quality})
	}

	if err != nil {
		return 0, 0, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	bounds := thumbnail.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}
