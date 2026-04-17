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

// Path returns the config file path.
func Path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(configDir, "convoy", "config.yaml"), nil
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

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

// EnsureExists creates the config file with defaults if it doesn't exist.
func EnsureExists() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	cfg := DefaultConfig()
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", fmt.Errorf("marshal default config: %w", err)
	}

	header := []byte("# convoy configuration\n# See: https://github.com/BloomerAB/convoy\n\n")
	if err := os.WriteFile(path, append(header, data...), 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return path, nil
}
