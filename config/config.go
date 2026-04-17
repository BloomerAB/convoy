package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RefreshInterval time.Duration `yaml:"-"`
	RawRefresh      string        `yaml:"refreshInterval"`
	Clusters        []Cluster     `yaml:"clusters"`
	GitHub          GitHubConfig  `yaml:"github"`
}

type Cluster struct {
	Context     string `yaml:"context"`
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
}

type GitHubConfig struct {
	Org     string   `yaml:"org"`
	Repos   []string `yaml:"repos"`
	MaxRuns int      `yaml:"maxRuns"`
	Token   string   `yaml:"token"`
}

func DefaultConfig() Config {
	return Config{
		RefreshInterval: 30 * time.Second,
		RawRefresh:      "30s",
		GitHub: GitHubConfig{
			MaxRuns: 5,
		},
	}
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	configDir, err := os.UserConfigDir()
	if err != nil {
		return cfg, fmt.Errorf("config dir: %w", err)
	}

	path := filepath.Join(configDir, "convoy", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	if cfg.RawRefresh != "" {
		d, err := time.ParseDuration(cfg.RawRefresh)
		if err != nil {
			return cfg, fmt.Errorf("parse refreshInterval %q: %w", cfg.RawRefresh, err)
		}
		cfg.RefreshInterval = d
	}

	if cfg.GitHub.MaxRuns == 0 {
		cfg.GitHub.MaxRuns = 5
	}

	return cfg, nil
}
