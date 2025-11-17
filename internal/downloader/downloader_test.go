package downloader

import (
	"testing"
)

func TestDetermineMediaType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		url         string
		expected    string
	}{
		// Image detection from content type
		{
			name:        "image from content type",
			contentType: "image/jpeg",
			url:         "https://example.com/file",
			expected:    "image",
		},
		{
			name:        "image/png content type",
			contentType: "image/png",
			url:         "https://example.com/file",
			expected:    "image",
		},
		// Image detection from URL extension
		{
			name:        "jpg extension",
			contentType: "",
			url:         "https://example.com/photo.jpg",
			expected:    "image",
		},
		{
			name:        "jpeg extension",
			contentType: "",
			url:         "https://example.com/photo.jpeg",
			expected:    "image",
		},
		{
			name:        "png extension",
			contentType: "",
			url:         "https://example.com/photo.png",
			expected:    "image",
		},
		{
			name:        "gif extension",
			contentType: "",
			url:         "https://example.com/photo.gif",
			expected:    "image",
		},
		{
			name:        "webp extension",
			contentType: "",
			url:         "https://example.com/photo.webp",
			expected:    "image",
		},
		{
			name:        "bmp extension",
			contentType: "",
			url:         "https://example.com/photo.bmp",
			expected:    "image",
		},
		// Video detection from content type
		{
			name:        "video from content type",
			contentType: "video/mp4",
			url:         "https://example.com/file",
			expected:    "video",
		},
		{
			name:        "video/webm content type",
			contentType: "video/webm",
			url:         "https://example.com/file",
			expected:    "video",
		},
		// Video detection from URL extension
		{
			name:        "mp4 extension",
			contentType: "",
			url:         "https://example.com/video.mp4",
			expected:    "video",
		},
		{
			name:        "webm extension",
			contentType: "",
			url:         "https://example.com/video.webm",
			expected:    "video",
		},
		{
			name:        "mov extension",
			contentType: "",
			url:         "https://example.com/video.mov",
			expected:    "video",
		},
		{
			name:        "avi extension",
			contentType: "",
			url:         "https://example.com/video.avi",
			expected:    "video",
		},
		{
			name:        "mkv extension",
			contentType: "",
			url:         "https://example.com/video.mkv",
			expected:    "video",
		},
		{
			name:        "m4v extension",
			contentType: "",
			url:         "https://example.com/video.m4v",
			expected:    "video",
		},
		// Case insensitive
		{
			name:        "uppercase JPG extension",
			contentType: "",
			url:         "https://example.com/PHOTO.JPG",
			expected:    "image",
		},
		{
			name:        "mixed case MP4 extension",
			contentType: "",
			url:         "https://example.com/VIDEO.Mp4",
			expected:    "video",
		},
		// Other media types
		{
			name:        "unknown type",
			contentType: "application/pdf",
			url:         "https://example.com/document.pdf",
			expected:    "other",
		},
		{
			name:        "no extension or content type",
			contentType: "",
			url:         "https://example.com/file",
			expected:    "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineMediaType(tt.contentType, tt.url)
			if result != tt.expected {
				t.Errorf("determineMediaType(%q, %q) = %q, want %q", tt.contentType, tt.url, result, tt.expected)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		url         string
		expected    string
	}{
		// Extension from URL
		{
			name:        "jpg from URL",
			contentType: "",
			url:         "https://example.com/photo.jpg",
			expected:    ".jpg",
		},
		{
			name:        "png from URL",
			contentType: "",
			url:         "https://example.com/photo.png",
			expected:    ".png",
		},
		{
			name:        "mp4 from URL",
			contentType: "",
			url:         "https://example.com/video.mp4",
			expected:    ".mp4",
		},
		{
			name:        "extension with query params",
			contentType: "",
			url:         "https://example.com/photo.jpg?size=large",
			expected:    ".jpg",
		},
		// Extension from content type when URL has none
		{
			name:        "jpg from content type",
			contentType: "image/jpeg",
			url:         "https://example.com/file",
			expected:    ".jpg",
		},
		{
			name:        "png from content type",
			contentType: "image/png",
			url:         "https://example.com/file",
			expected:    ".png",
		},
		{
			name:        "gif from content type",
			contentType: "image/gif",
			url:         "https://example.com/file",
			expected:    ".gif",
		},
		{
			name:        "webp from content type",
			contentType: "image/webp",
			url:         "https://example.com/file",
			expected:    ".webp",
		},
		{
			name:        "mp4 from content type",
			contentType: "video/mp4",
			url:         "https://example.com/file",
			expected:    ".mp4",
		},
		{
			name:        "webm from content type",
			contentType: "video/webm",
			url:         "https://example.com/file",
			expected:    ".webm",
		},
		// URL extension takes precedence
		{
			name:        "URL extension overrides content type",
			contentType: "image/jpeg",
			url:         "https://example.com/photo.png",
			expected:    ".png",
		},
		// Unknown types
		{
			name:        "unknown content type",
			contentType: "application/octet-stream",
			url:         "https://example.com/file",
			expected:    ".bin",
		},
		{
			name:        "no extension or content type",
			contentType: "",
			url:         "https://example.com/file",
			expected:    ".bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileExtension(tt.contentType, tt.url)
			if result != tt.expected {
				t.Errorf("getFileExtension(%q, %q) = %q, want %q", tt.contentType, tt.url, result, tt.expected)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean path",
			input:    "technology",
			expected: "technology",
		},
		{
			name:     "path with slash",
			input:    "tech/programming",
			expected: "tech_programming",
		},
		{
			name:     "path with backslash",
			input:    "tech\\programming",
			expected: "tech_programming",
		},
		{
			name:     "path with colon",
			input:    "tech:programming",
			expected: "tech_programming",
		},
		{
			name:     "path with asterisk",
			input:    "tech*programming",
			expected: "tech_programming",
		},
		{
			name:     "path with question mark",
			input:    "tech?programming",
			expected: "tech_programming",
		},
		{
			name:     "path with quotes",
			input:    "tech\"programming",
			expected: "tech_programming",
		},
		{
			name:     "path with angle brackets",
			input:    "tech<programming>",
			expected: "tech_programming_",
		},
		{
			name:     "path with pipe",
			input:    "tech|programming",
			expected: "tech_programming",
		},
		{
			name:     "multiple invalid characters",
			input:    "tech/prog:ram*ming?",
			expected: "tech_prog_ram_ming_",
		},
		{
			name:     "path with @ symbol (valid)",
			input:    "technology@lemmy.ml",
			expected: "technology@lemmy.ml",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePath(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldDownload(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		includeImages  bool
		includeVideos  bool
		includeOther   bool
		expected       bool
	}{
		// Image URLs
		{
			name:           "download image when images enabled",
			url:            "https://example.com/photo.jpg",
			includeImages:  true,
			includeVideos:  false,
			includeOther:   false,
			expected:       true,
		},
		{
			name:           "skip image when images disabled",
			url:            "https://example.com/photo.jpg",
			includeImages:  false,
			includeVideos:  true,
			includeOther:   true,
			expected:       false,
		},
		{
			name:           "download png when images enabled",
			url:            "https://example.com/photo.png",
			includeImages:  true,
			includeVideos:  false,
			includeOther:   false,
			expected:       true,
		},
		// Video URLs
		{
			name:           "download video when videos enabled",
			url:            "https://example.com/video.mp4",
			includeImages:  false,
			includeVideos:  true,
			includeOther:   false,
			expected:       true,
		},
		{
			name:           "skip video when videos disabled",
			url:            "https://example.com/video.mp4",
			includeImages:  true,
			includeVideos:  false,
			includeOther:   true,
			expected:       false,
		},
		{
			name:           "download webm when videos enabled",
			url:            "https://example.com/video.webm",
			includeImages:  false,
			includeVideos:  true,
			includeOther:   false,
			expected:       true,
		},
		// Other media types
		{
			name:           "download other when enabled",
			url:            "https://example.com/file.pdf",
			includeImages:  false,
			includeVideos:  false,
			includeOther:   true,
			expected:       true,
		},
		{
			name:           "skip other when disabled",
			url:            "https://example.com/file.pdf",
			includeImages:  true,
			includeVideos:  true,
			includeOther:   false,
			expected:       false,
		},
		// All enabled
		{
			name:           "download image when all enabled",
			url:            "https://example.com/photo.jpg",
			includeImages:  true,
			includeVideos:  true,
			includeOther:   true,
			expected:       true,
		},
		{
			name:           "download video when all enabled",
			url:            "https://example.com/video.mp4",
			includeImages:  true,
			includeVideos:  true,
			includeOther:   true,
			expected:       true,
		},
		// All disabled
		{
			name:           "skip image when all disabled",
			url:            "https://example.com/photo.jpg",
			includeImages:  false,
			includeVideos:  false,
			includeOther:   false,
			expected:       false,
		},
		// Unknown URL type
		{
			name:           "unknown type defaults to other",
			url:            "https://example.com/unknown",
			includeImages:  false,
			includeVideos:  false,
			includeOther:   true,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldDownload(tt.url, tt.includeImages, tt.includeVideos, tt.includeOther)
			if result != tt.expected {
				t.Errorf("ShouldDownload(%q, images=%v, videos=%v, other=%v) = %v, want %v",
					tt.url, tt.includeImages, tt.includeVideos, tt.includeOther, result, tt.expected)
			}
		})
	}
}
