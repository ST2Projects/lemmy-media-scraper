package thumbnails

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// createTestImage creates a small solid-color PNG file at the given path.
func createTestImage(t *testing.T, path string, width, height int) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir for test image: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create test image file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
}

func TestGenerateImageThumbnail(t *testing.T) {
	tmpDir := t.TempDir()
	thumbDir := filepath.Join(tmpDir, "thumbs")

	gen := NewGenerator(200, 200, 85, thumbDir, "ffmpeg")

	// Create a source image larger than thumbnail dimensions
	srcPath := filepath.Join(tmpDir, "source.png")
	createTestImage(t, srcPath, 800, 600)

	tests := []struct {
		name       string
		mediaPath  string
		mediaType  string
		wantErr    bool
		checkSize  bool
		maxWidth   int
		maxHeight  int
	}{
		{
			name:      "valid image",
			mediaPath: srcPath,
			mediaType: "image/png",
			wantErr:   false,
			checkSize: true,
			maxWidth:  200,
			maxHeight: 200,
		},
		{
			name:      "nonexistent image",
			mediaPath: filepath.Join(tmpDir, "nonexistent.png"),
			mediaType: "image/png",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thumbPath, width, height, err := gen.GenerateThumbnail(tt.mediaPath, tt.mediaType)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GenerateThumbnail() error = %v", err)
			}

			if thumbPath == "" {
				t.Error("thumbnail path is empty")
			}

			// Verify thumbnail file exists
			if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
				t.Error("thumbnail file does not exist")
			}

			if tt.checkSize {
				if width > tt.maxWidth {
					t.Errorf("width = %d, exceeds max %d", width, tt.maxWidth)
				}
				if height > tt.maxHeight {
					t.Errorf("height = %d, exceeds max %d", height, tt.maxHeight)
				}
				if width == 0 || height == 0 {
					t.Error("thumbnail dimensions should be non-zero")
				}
			}
		})
	}
}

func TestGenerateImageThumbnailSmallImage(t *testing.T) {
	tmpDir := t.TempDir()
	thumbDir := filepath.Join(tmpDir, "thumbs")
	gen := NewGenerator(400, 400, 85, thumbDir, "ffmpeg")

	// Create a source image smaller than max dimensions
	srcPath := filepath.Join(tmpDir, "small.png")
	createTestImage(t, srcPath, 100, 50)

	thumbPath, width, height, err := gen.GenerateThumbnail(srcPath, "image/png")
	if err != nil {
		t.Fatalf("GenerateThumbnail() error = %v", err)
	}

	if thumbPath == "" {
		t.Error("thumbnail path is empty")
	}

	// Image is smaller than max, so output should be at most original size
	if width > 100 {
		t.Errorf("width = %d, should not exceed original 100", width)
	}
	if height > 50 {
		t.Errorf("height = %d, should not exceed original 50", height)
	}
}

func TestThumbnailExists(t *testing.T) {
	tmpDir := t.TempDir()
	thumbDir := filepath.Join(tmpDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		t.Fatalf("failed to create thumb dir: %v", err)
	}

	gen := NewGenerator(200, 200, 85, thumbDir, "ffmpeg")

	tests := []struct {
		name       string
		setup      func()
		mediaPath  string
		wantExists bool
	}{
		{
			name:       "thumbnail does not exist",
			setup:      func() {},
			mediaPath:  "/some/path/image.png",
			wantExists: false,
		},
		{
			name: "thumbnail exists",
			setup: func() {
				// Create a file at the expected thumbnail path
				thumbPath := filepath.Join(thumbDir, "image.jpg")
				os.WriteFile(thumbPath, []byte("thumb data"), 0644)
			},
			mediaPath:  "/some/path/image.png",
			wantExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got := gen.ThumbnailExists(tt.mediaPath)
			if got != tt.wantExists {
				t.Errorf("ThumbnailExists() = %v, want %v", got, tt.wantExists)
			}
		})
	}
}

func TestGetThumbnailPath(t *testing.T) {
	gen := NewGenerator(200, 200, 85, "/tmp/thumbs", "ffmpeg")

	tests := []struct {
		name      string
		mediaPath string
		want      string
	}{
		{
			name:      "jpg image",
			mediaPath: "/media/pics/photo.jpg",
			want:      "/tmp/thumbs/photo.jpg",
		},
		{
			name:      "png image",
			mediaPath: "/media/pics/screenshot.png",
			want:      "/tmp/thumbs/screenshot.jpg",
		},
		{
			name:      "video file",
			mediaPath: "/media/vids/clip.mp4",
			want:      "/tmp/thumbs/clip.jpg",
		},
		{
			name:      "nested path",
			mediaPath: "/a/b/c/d/image.webp",
			want:      "/tmp/thumbs/image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.GetThumbnailPath(tt.mediaPath)
			if got != tt.want {
				t.Errorf("GetThumbnailPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateThumbnailUnsupportedType(t *testing.T) {
	tmpDir := t.TempDir()
	thumbDir := filepath.Join(tmpDir, "thumbs")
	gen := NewGenerator(200, 200, 85, thumbDir, "ffmpeg")

	srcPath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(srcPath, []byte("not media"), 0644)

	_, _, _, err := gen.GenerateThumbnail(srcPath, "text/plain")
	if err == nil {
		t.Error("expected error for unsupported media type, got nil")
	}
	if err != nil && !contains(err.Error(), "unsupported media type") {
		t.Errorf("error = %q, want it to contain 'unsupported media type'", err.Error())
	}
}

func TestGenerateThumbnailDirCreation(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a deeply nested path that doesn't exist yet
	thumbDir := filepath.Join(tmpDir, "deeply", "nested", "thumbdir")
	gen := NewGenerator(200, 200, 85, thumbDir, "ffmpeg")

	srcPath := filepath.Join(tmpDir, "source.png")
	createTestImage(t, srcPath, 100, 100)

	thumbPath, _, _, err := gen.GenerateThumbnail(srcPath, "image/png")
	if err != nil {
		t.Fatalf("GenerateThumbnail() error = %v", err)
	}

	// Verify the directory was created
	if _, err := os.Stat(thumbDir); os.IsNotExist(err) {
		t.Error("thumbnail directory was not created")
	}

	// Verify the thumbnail file exists
	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		t.Error("thumbnail file was not created")
	}
}

func TestGenerateThumbnailCaching(t *testing.T) {
	tmpDir := t.TempDir()
	thumbDir := filepath.Join(tmpDir, "thumbs")
	gen := NewGenerator(200, 200, 85, thumbDir, "ffmpeg")

	srcPath := filepath.Join(tmpDir, "source.png")
	createTestImage(t, srcPath, 400, 300)

	// First generation
	path1, w1, h1, err := gen.GenerateThumbnail(srcPath, "image/png")
	if err != nil {
		t.Fatalf("first GenerateThumbnail() error = %v", err)
	}

	// Second call should return cached result (same path)
	path2, w2, h2, err := gen.GenerateThumbnail(srcPath, "image/png")
	if err != nil {
		t.Fatalf("second GenerateThumbnail() error = %v", err)
	}

	if path1 != path2 {
		t.Errorf("cached path mismatch: %q != %q", path1, path2)
	}
	if w1 != w2 || h1 != h2 {
		t.Errorf("cached dimensions mismatch: %dx%d != %dx%d", w1, h1, w2, h2)
	}
}

// contains checks if s contains substr. Avoids importing strings for a test helper.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
