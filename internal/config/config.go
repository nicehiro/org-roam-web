package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Site    SiteConfig    `yaml:"site"`
	Paths   PathsConfig   `yaml:"paths"`
	Exclude ExcludeConfig `yaml:"exclude"`
	Display DisplayConfig `yaml:"display"`
}

type SiteConfig struct {
	Title   string `yaml:"title"`
	BaseURL string `yaml:"base_url"`
}

type PathsConfig struct {
	RoamDir   string `yaml:"roam_dir"`
	DBPath    string `yaml:"db_path"`
	OutputDir string `yaml:"output_dir"`
}

type ExcludeConfig struct {
	Tags  []string `yaml:"tags"`
	Files []string `yaml:"files"`
	IDs   []string `yaml:"ids"`
}

type DisplayConfig struct {
	RecentCount     int `yaml:"recent_count"`
	LocalGraphDepth int `yaml:"local_graph_depth"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Site: SiteConfig{
			Title:   "My Notes",
			BaseURL: "",
		},
		Paths: PathsConfig{
			RoamDir:   ".",
			DBPath:    "./roam.db",
			OutputDir: "./dist",
		},
		Exclude: ExcludeConfig{
			Tags:  []string{"private", "draft"},
			Files: []string{},
			IDs:   []string{},
		},
		Display: DisplayConfig{
			RecentCount:     20,
			LocalGraphDepth: 2,
		},
	}
}

// Load reads config from a YAML file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Expand paths
	cfg.Paths.RoamDir = expandPath(cfg.Paths.RoamDir)
	cfg.Paths.DBPath = expandPath(cfg.Paths.DBPath)
	cfg.Paths.OutputDir = expandPath(cfg.Paths.OutputDir)

	return cfg, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
