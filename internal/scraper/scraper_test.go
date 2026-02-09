package scraper

import (
	"testing"

	"github.com/ST2Projects/lemmy-media-scraper/pkg/models"
)

func TestIsMediaURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Image extensions
		{name: "jpg extension", url: "https://example.com/photo.jpg", want: true},
		{name: "jpeg extension", url: "https://example.com/photo.jpeg", want: true},
		{name: "png extension", url: "https://example.com/image.png", want: true},
		{name: "gif extension", url: "https://example.com/anim.gif", want: true},
		{name: "webp extension", url: "https://example.com/image.webp", want: true},
		{name: "bmp extension", url: "https://example.com/image.bmp", want: true},
		{name: "svg extension", url: "https://example.com/icon.svg", want: true},

		// Video extensions
		{name: "mp4 extension", url: "https://example.com/video.mp4", want: true},
		{name: "webm extension", url: "https://example.com/video.webm", want: true},
		{name: "mov extension", url: "https://example.com/video.mov", want: true},
		{name: "avi extension", url: "https://example.com/video.avi", want: true},
		{name: "mkv extension", url: "https://example.com/video.mkv", want: true},
		{name: "m4v extension", url: "https://example.com/video.m4v", want: true},
		{name: "flv extension", url: "https://example.com/video.flv", want: true},

		// Media hosts
		{name: "imgur host", url: "https://i.imgur.com/abc123", want: true},
		{name: "reddit image host", url: "https://i.redd.it/xyz789.png", want: true},
		{name: "reddit video host", url: "https://v.redd.it/abc", want: true},
		{name: "reddit preview", url: "https://preview.redd.it/image.jpg", want: true},
		{name: "reddit external preview", url: "https://external-preview.redd.it/image", want: true},
		{name: "lemmy pictrs", url: "https://lemmy.world/pictrs/image/abc.webp", want: true},
		{name: "pictrs generic", url: "https://someinstance.com/pictrs/image/abc", want: true},

		// Case insensitive
		{name: "uppercase extension", url: "https://example.com/photo.JPG", want: true},
		{name: "mixed case extension", url: "https://example.com/photo.JpG", want: true},

		// Extension in query string
		{name: "extension in query", url: "https://example.com/media?file=photo.jpg", want: true},

		// Non-media URLs
		{name: "html page", url: "https://example.com/page.html", want: false},
		{name: "plain URL", url: "https://example.com/some/path", want: false},
		{name: "text file", url: "https://example.com/readme.txt", want: false},
		{name: "json endpoint", url: "https://api.example.com/data.json", want: false},
		{name: "empty URL", url: "", want: false},
		{name: "just a domain", url: "https://example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMediaURL(tt.url)
			if got != tt.want {
				t.Errorf("isMediaURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractMediaURLs(t *testing.T) {
	s := &Scraper{} // extractMediaURLs doesn't use any Scraper fields except method receiver

	tests := []struct {
		name     string
		postView models.PostView
		want     []string
	}{
		{
			name: "main URL only (image)",
			postView: models.PostView{
				Post: models.Post{
					URL: "https://example.com/photo.jpg",
				},
			},
			want: []string{"https://example.com/photo.jpg"},
		},
		{
			name: "main URL and embed video (different content)",
			postView: models.PostView{
				Post: models.Post{
					URL:           "https://example.com/photo.jpg",
					EmbedVideoURL: "https://example.com/video.mp4",
				},
			},
			want: []string{"https://example.com/photo.jpg", "https://example.com/video.mp4"},
		},
		{
			name: "main URL present - thumbnail skipped",
			postView: models.PostView{
				Post: models.Post{
					URL:          "https://example.com/photo.jpg",
					ThumbnailURL: "https://example.com/thumb.jpg",
				},
			},
			want: []string{"https://example.com/photo.jpg"},
		},
		{
			name: "no main URL - embed video used",
			postView: models.PostView{
				Post: models.Post{
					EmbedVideoURL: "https://example.com/video.mp4",
				},
			},
			want: []string{"https://example.com/video.mp4"},
		},
		{
			name: "no main URL - no embed - thumbnail fallback",
			postView: models.PostView{
				Post: models.Post{
					ThumbnailURL: "https://example.com/thumb.jpg",
				},
			},
			want: []string{"https://example.com/thumb.jpg"},
		},
		{
			name: "no media URLs at all",
			postView: models.PostView{
				Post: models.Post{
					URL: "https://example.com/article",
				},
			},
			want: nil,
		},
		{
			name:     "empty post",
			postView: models.PostView{},
			want:     nil,
		},
		{
			name: "non-media main URL with media thumbnail",
			postView: models.PostView{
				Post: models.Post{
					URL:          "https://news.example.com/article",
					ThumbnailURL: "https://example.com/thumb.png",
				},
			},
			want: []string{"https://example.com/thumb.png"},
		},
		{
			name: "non-media embed video skipped",
			postView: models.PostView{
				Post: models.Post{
					EmbedVideoURL: "https://youtube.com/watch?v=abc",
				},
			},
			want: nil,
		},
		{
			name: "main URL is non-media but embed video is media",
			postView: models.PostView{
				Post: models.Post{
					URL:           "https://example.com/article",
					EmbedVideoURL: "https://v.redd.it/video123",
				},
			},
			want: []string{"https://v.redd.it/video123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.extractMediaURLs(tt.postView)

			if len(got) != len(tt.want) {
				t.Fatalf("extractMediaURLs() returned %d URLs, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("URL[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
