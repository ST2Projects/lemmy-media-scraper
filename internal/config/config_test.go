package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: false,
		},
		{
			name: "missing instance",
			config: Config{
				Lemmy: LemmyConfig{
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: true,
			errMsg:  "lemmy.instance is required",
		},
		{
			name: "missing username",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: true,
			errMsg:  "lemmy.username is required",
		},
		{
			name: "missing password",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: true,
			errMsg:  "lemmy.password is required",
		},
		{
			name: "missing base directory",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: true,
			errMsg:  "storage.base_directory is required",
		},
		{
			name: "missing database path",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
			},
			wantErr: true,
			errMsg:  "database.path is required",
		},
		{
			name: "invalid run mode",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "run_mode.mode must be 'once' or 'continuous'",
		},
		{
			name: "continuous mode without interval",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode: "continuous",
				},
			},
			wantErr: true,
			errMsg:  "run_mode.interval is required for continuous mode",
		},
		{
			name: "valid continuous mode with interval",
			config: Config{
				Lemmy: LemmyConfig{
					Instance: "lemmy.ml",
					Username: "testuser",
					Password: "testpass",
				},
				Storage: StorageConfig{
					BaseDirectory: "/tmp/media",
				},
				Database: DatabaseConfig{
					Path: "/tmp/db.sqlite",
				},
				RunMode: RunModeConfig{
					Mode:     "continuous",
					Interval: 5 * time.Minute,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name:   "empty scraper config gets defaults",
			config: Config{},
			expected: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:     50,
					SeenPostsThreshold: 5,
					SortType:           "Hot",
					IncludeImages:      true,
					IncludeVideos:      true,
					IncludeOtherMedia:  true,
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
				WebServer: WebServerConfig{
					Port: 8080,
					Host: "localhost",
				},
			},
		},
		{
			name: "pagination disabled with high max posts caps at 50",
			config: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:   100,
					EnablePagination: false,
				},
			},
			expected: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:     50,
					EnablePagination:   false,
					SeenPostsThreshold: 5,
					SortType:           "Hot",
					IncludeImages:      true,
					IncludeVideos:      true,
					IncludeOtherMedia:  true,
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
				WebServer: WebServerConfig{
					Port: 8080,
					Host: "localhost",
				},
			},
		},
		{
			name: "pagination enabled allows more than 50 posts",
			config: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:   100,
					EnablePagination: true,
				},
			},
			expected: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:     100,
					EnablePagination:   true,
					SeenPostsThreshold: 5,
					SortType:           "Hot",
					IncludeImages:      true,
					IncludeVideos:      true,
					IncludeOtherMedia:  true,
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
				WebServer: WebServerConfig{
					Port: 8080,
					Host: "localhost",
				},
			},
		},
		{
			name: "existing media type preferences preserved",
			config: Config{
				Scraper: ScraperConfig{
					IncludeImages: true,
					IncludeVideos: false,
				},
			},
			expected: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:     50,
					SeenPostsThreshold: 5,
					SortType:           "Hot",
					IncludeImages:      true,
					IncludeVideos:      false,
					IncludeOtherMedia:  false,
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
				WebServer: WebServerConfig{
					Port: 8080,
					Host: "localhost",
				},
			},
		},
		{
			name: "custom web server port and host preserved",
			config: Config{
				WebServer: WebServerConfig{
					Port: 9000,
					Host: "0.0.0.0",
				},
			},
			expected: Config{
				Scraper: ScraperConfig{
					MaxPostsPerRun:     50,
					SeenPostsThreshold: 5,
					SortType:           "Hot",
					IncludeImages:      true,
					IncludeVideos:      true,
					IncludeOtherMedia:  true,
				},
				RunMode: RunModeConfig{
					Mode: "once",
				},
				WebServer: WebServerConfig{
					Port: 9000,
					Host: "0.0.0.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			if tt.config.Scraper.MaxPostsPerRun != tt.expected.Scraper.MaxPostsPerRun {
				t.Errorf("MaxPostsPerRun = %d, want %d", tt.config.Scraper.MaxPostsPerRun, tt.expected.Scraper.MaxPostsPerRun)
			}
			if tt.config.Scraper.SeenPostsThreshold != tt.expected.Scraper.SeenPostsThreshold {
				t.Errorf("SeenPostsThreshold = %d, want %d", tt.config.Scraper.SeenPostsThreshold, tt.expected.Scraper.SeenPostsThreshold)
			}
			if tt.config.Scraper.SortType != tt.expected.Scraper.SortType {
				t.Errorf("SortType = %s, want %s", tt.config.Scraper.SortType, tt.expected.Scraper.SortType)
			}
			if tt.config.Scraper.IncludeImages != tt.expected.Scraper.IncludeImages {
				t.Errorf("IncludeImages = %v, want %v", tt.config.Scraper.IncludeImages, tt.expected.Scraper.IncludeImages)
			}
			if tt.config.Scraper.IncludeVideos != tt.expected.Scraper.IncludeVideos {
				t.Errorf("IncludeVideos = %v, want %v", tt.config.Scraper.IncludeVideos, tt.expected.Scraper.IncludeVideos)
			}
			if tt.config.Scraper.IncludeOtherMedia != tt.expected.Scraper.IncludeOtherMedia {
				t.Errorf("IncludeOtherMedia = %v, want %v", tt.config.Scraper.IncludeOtherMedia, tt.expected.Scraper.IncludeOtherMedia)
			}
			if tt.config.RunMode.Mode != tt.expected.RunMode.Mode {
				t.Errorf("RunMode.Mode = %s, want %s", tt.config.RunMode.Mode, tt.expected.RunMode.Mode)
			}
			if tt.config.WebServer.Port != tt.expected.WebServer.Port {
				t.Errorf("WebServer.Port = %d, want %d", tt.config.WebServer.Port, tt.expected.WebServer.Port)
			}
			if tt.config.WebServer.Host != tt.expected.WebServer.Host {
				t.Errorf("WebServer.Host = %s, want %s", tt.config.WebServer.Host, tt.expected.WebServer.Host)
			}
		})
	}
}

func TestNormalizeSortType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hot", "Hot"},
		{"Hot", "Hot"},
		{"new", "New"},
		{"New", "New"},
		{"topday", "TopDay"},
		{"TopDay", "TopDay"},
		{"topweek", "TopWeek"},
		{"TopWeek", "TopWeek"},
		{"topmonth", "TopMonth"},
		{"TopMonth", "TopMonth"},
		{"topyear", "TopYear"},
		{"TopYear", "TopYear"},
		{"topall", "TopAll"},
		{"TopAll", "TopAll"},
		{"active", "Active"},
		{"Active", "Active"},
		{"UnknownSort", "UnknownSort"}, // Unknown values pass through
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSortType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSortType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "valid minimal config",
			yaml: `
lemmy:
  instance: "lemmy.ml"
  username: "testuser"
  password: "testpass"
storage:
  base_directory: "/tmp/media"
database:
  path: "/tmp/db.sqlite"
run_mode:
  mode: "once"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if c.Lemmy.Instance != "lemmy.ml" {
					t.Errorf("Instance = %s, want lemmy.ml", c.Lemmy.Instance)
				}
				if c.Lemmy.Username != "testuser" {
					t.Errorf("Username = %s, want testuser", c.Lemmy.Username)
				}
			},
		},
		{
			name: "config with communities",
			yaml: `
lemmy:
  instance: "lemmy.ml"
  username: "testuser"
  password: "testpass"
  communities:
    - "technology"
    - "programming"
storage:
  base_directory: "/tmp/media"
database:
  path: "/tmp/db.sqlite"
run_mode:
  mode: "once"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Lemmy.Communities) != 2 {
					t.Errorf("Communities length = %d, want 2", len(c.Lemmy.Communities))
				}
				if c.Lemmy.Communities[0] != "technology" {
					t.Errorf("Communities[0] = %s, want technology", c.Lemmy.Communities[0])
				}
			},
		},
		{
			name: "invalid yaml",
			yaml: `
invalid: yaml: content:
  - this is broken
`,
			wantErr: true,
		},
		{
			name: "missing required field",
			yaml: `
lemmy:
  instance: "lemmy.ml"
  # username missing
  password: "testpass"
storage:
  base_directory: "/tmp/media"
database:
  path: "/tmp/db.sqlite"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := LoadConfig(configPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("LoadConfig() unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestLoadConfigNonexistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Errorf("LoadConfig() with nonexistent file should return error")
	}
}
