package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Lemmy       LemmyConfig       `yaml:"lemmy" json:"lemmy"`
	Storage     StorageConfig     `yaml:"storage" json:"storage"`
	Database    DatabaseConfig    `yaml:"database" json:"database"`
	Scraper     ScraperConfig     `yaml:"scraper" json:"scraper"`
	RunMode     RunModeConfig     `yaml:"run_mode" json:"run_mode"`
	WebServer   WebServerConfig   `yaml:"web_server" json:"web_server"`
	Thumbnails  ThumbnailConfig   `yaml:"thumbnails" json:"thumbnails"`
	Recognition RecognitionConfig `yaml:"recognition" json:"recognition"`
	Search      SearchConfig      `yaml:"search" json:"search"`
}

// LemmyConfig contains Lemmy instance and authentication settings
type LemmyConfig struct {
	Instance    string   `yaml:"instance" json:"instance"`        // e.g., "lemmy.ml"
	Username    string   `yaml:"username" json:"username"`
	Password    string   `yaml:"password" json:"password"`
	Communities []string `yaml:"communities" json:"communities"`  // Optional list of communities to scrape
}

// StorageConfig contains settings for media storage
type StorageConfig struct {
	BaseDirectory string `yaml:"base_directory" json:"base_directory"`  // Where to save downloaded media
}

// DatabaseConfig contains SQLite database settings
type DatabaseConfig struct {
	Path string `yaml:"path" json:"path"`  // Path to SQLite database file
}

// ScraperConfig contains scraping behavior settings
type ScraperConfig struct {
	MaxPostsPerRun         int    `yaml:"max_posts_per_run" json:"max_posts_per_run"`           // Maximum posts to scrape per run (total across all pages)
	StopAtSeenPosts        bool   `yaml:"stop_at_seen_posts" json:"stop_at_seen_posts"`         // Stop when encountering previously seen posts
	SkipSeenPosts          bool   `yaml:"skip_seen_posts" json:"skip_seen_posts"`               // Skip seen posts but continue scraping (vs stopping)
	EnablePagination       bool   `yaml:"enable_pagination" json:"enable_pagination"`           // Fetch multiple pages to get more than 50 posts
	SeenPostsThreshold     int    `yaml:"seen_posts_threshold" json:"seen_posts_threshold"`     // Stop after encountering this many seen posts in a row
	SortType               string `yaml:"sort_type" json:"sort_type"`                           // e.g., "Hot", "New", "TopDay"
	IncludeImages          bool   `yaml:"include_images" json:"include_images"`                 // Download images
	IncludeVideos          bool   `yaml:"include_videos" json:"include_videos"`                 // Download videos
	IncludeOtherMedia      bool   `yaml:"include_other_media" json:"include_other_media"`       // Download other media types
}

// RunModeConfig contains run mode settings
type RunModeConfig struct {
	Mode     string        `yaml:"mode" json:"mode"`          // "once" or "continuous"
	Interval time.Duration `yaml:"interval" json:"interval"`  // Interval for continuous mode (e.g., "5m", "1h")
}

// WebServerConfig contains web UI server settings
type WebServerConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`  // Enable web UI server
	Host    string `yaml:"host" json:"host"`        // Host to bind to (e.g., "localhost", "0.0.0.0")
	Port    int    `yaml:"port" json:"port"`        // Port to listen on
}

// ThumbnailConfig contains thumbnail generation settings
type ThumbnailConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`             // Enable thumbnail generation
	MaxWidth    int    `yaml:"max_width" json:"max_width"`         // Maximum thumbnail width
	MaxHeight   int    `yaml:"max_height" json:"max_height"`       // Maximum thumbnail height
	Quality     int    `yaml:"quality" json:"quality"`             // JPEG quality (1-100)
	Directory   string `yaml:"directory" json:"directory"`         // Directory to store thumbnails
	VideoMethod string `yaml:"video_method" json:"video_method"`   // Method for video thumbnails (ffmpeg, frame_extract)
}

// RecognitionConfig contains image recognition settings
type RecognitionConfig struct {
	Enabled            bool    `yaml:"enabled" json:"enabled"`                           // Enable image recognition
	Provider           string  `yaml:"provider" json:"provider"`                         // Recognition provider (ollama, none)
	OllamaURL          string  `yaml:"ollama_url" json:"ollama_url"`                     // Ollama API URL
	Model              string  `yaml:"model" json:"model"`                               // Model to use (e.g., llama3.2-vision:latest)
	AutoTag            bool    `yaml:"auto_tag" json:"auto_tag"`                         // Automatically create tags from classifications
	NSFWDetection      bool    `yaml:"nsfw_detection" json:"nsfw_detection"`             // Enable NSFW content detection
	ConfidenceThreshold float64 `yaml:"confidence_threshold" json:"confidence_threshold"` // Minimum confidence for auto-tagging (0.0-1.0)
}

// SearchConfig contains search settings
type SearchConfig struct {
	RebuildIndex bool `yaml:"rebuild_index" json:"rebuild_index"` // Rebuild FTS index on startup
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(path string, config *Config) error {
	// Validate before saving
	if err := config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Lemmy.Instance == "" {
		return fmt.Errorf("lemmy.instance is required")
	}
	if c.Lemmy.Username == "" {
		return fmt.Errorf("lemmy.username is required")
	}
	if c.Lemmy.Password == "" {
		return fmt.Errorf("lemmy.password is required")
	}
	if c.Storage.BaseDirectory == "" {
		return fmt.Errorf("storage.base_directory is required")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}
	if c.RunMode.Mode != "once" && c.RunMode.Mode != "continuous" {
		return fmt.Errorf("run_mode.mode must be 'once' or 'continuous'")
	}
	if c.RunMode.Mode == "continuous" && c.RunMode.Interval == 0 {
		return fmt.Errorf("run_mode.interval is required for continuous mode")
	}
	return nil
}

// SetDefaults sets default values for optional configuration fields
func (c *Config) SetDefaults() {
	if c.Scraper.MaxPostsPerRun == 0 {
		c.Scraper.MaxPostsPerRun = 50
	}

	// Set default threshold for seen posts
	if c.Scraper.SeenPostsThreshold == 0 {
		c.Scraper.SeenPostsThreshold = 5 // Stop after seeing 5 posts in a row we've already processed
	}

	// If pagination is disabled, limit to 50 (API max per request)
	if !c.Scraper.EnablePagination && c.Scraper.MaxPostsPerRun > 50 {
		c.Scraper.MaxPostsPerRun = 50
	}

	if c.Scraper.SortType == "" {
		c.Scraper.SortType = "Hot"
	}
	// Normalize sort type to match Lemmy API expectations
	c.Scraper.SortType = normalizeSortType(c.Scraper.SortType)

	if !c.Scraper.IncludeImages && !c.Scraper.IncludeVideos && !c.Scraper.IncludeOtherMedia {
		c.Scraper.IncludeImages = true
		c.Scraper.IncludeVideos = true
		c.Scraper.IncludeOtherMedia = true
	}
	if c.RunMode.Mode == "" {
		c.RunMode.Mode = "once"
	}

	// Web server defaults - enabled by default
	if c.WebServer.Port == 0 {
		c.WebServer.Port = 8080
	}
	if c.WebServer.Host == "" {
		c.WebServer.Host = "localhost"
	}
	// Enable web server by default (can be disabled via CLI flag)
	c.WebServer.Enabled = true

	// Thumbnail defaults
	if c.Thumbnails.MaxWidth == 0 {
		c.Thumbnails.MaxWidth = 400
	}
	if c.Thumbnails.MaxHeight == 0 {
		c.Thumbnails.MaxHeight = 400
	}
	if c.Thumbnails.Quality == 0 {
		c.Thumbnails.Quality = 85
	}
	if c.Thumbnails.Directory == "" {
		c.Thumbnails.Directory = "./thumbnails"
	}
	if c.Thumbnails.VideoMethod == "" {
		c.Thumbnails.VideoMethod = "ffmpeg"
	}

	// Recognition defaults
	if c.Recognition.Provider == "" {
		c.Recognition.Provider = "ollama"
	}
	if c.Recognition.OllamaURL == "" {
		c.Recognition.OllamaURL = "http://localhost:11434"
	}
	if c.Recognition.Model == "" {
		c.Recognition.Model = "llama3.2-vision:latest"
	}
	if c.Recognition.ConfidenceThreshold == 0 {
		c.Recognition.ConfidenceThreshold = 0.6
	}
}

// normalizeSortType converts user-friendly sort type names to API format
func normalizeSortType(sort string) string {
	// Map common variations to the correct API format
	// Based on Lemmy's SortType enum
	sortMap := map[string]string{
		"hot":      "Hot",
		"Hot":      "Hot",
		"new":      "New",
		"New":      "New",
		"topday":   "TopDay",
		"TopDay":   "TopDay",
		"topweek":  "TopWeek",
		"TopWeek":  "TopWeek",
		"topmonth": "TopMonth",
		"TopMonth": "TopMonth",
		"topyear":  "TopYear",
		"TopYear":  "TopYear",
		"topall":   "TopAll",
		"TopAll":   "TopAll",
		"active":   "Active",
		"Active":   "Active",
	}

	if normalized, ok := sortMap[sort]; ok {
		return normalized
	}
	return sort
}
