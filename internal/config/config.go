package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Remote is a push target.
type Remote struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// HealthConfig controls the health engine.
type HealthConfig struct {
	Statuspage bool     `yaml:"statuspage"` // consult githubstatus.com
	Endpoints  []string `yaml:"endpoints"`  // own HTTP checks
}

// Config is the fujin configuration.
type Config struct {
	Primary  Remote       `yaml:"primary"`
	Failover []Remote     `yaml:"failover"`
	Health   HealthConfig `yaml:"health"`
	DBPath   string       `yaml:"db_path"`
	// TelegramBotToken and TelegramChatID enable Telegram alerts for
	// queued/flushed pushes (e.g. "your push is QUEUED — GitHub is down").
	TelegramBotToken string `yaml:"telegram_bot_token"`
	TelegramChatID   string `yaml:"telegram_chat_id"`
}

// Default returns the built-in defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Health: HealthConfig{
			Statuspage: true,
			Endpoints:  []string{"https://github.com", "https://api.github.com"},
		},
		DBPath: filepath.Join(home, ".local", "share", "fujin", "fujin.db"),
	}
}

// DefaultPath returns the conventional config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "fujin", "config.yaml")
	}
	return filepath.Join(home, ".config", "fujin", "config.yaml")
}

// Save writes the config as YAML to path, creating parent directories.
// Used by `fujin init` to write the generated configuration.
func Save(path string, cfg *Config) error {
	if path == "" {
		path = DefaultPath()
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: create dir: %w", err)
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// Load reads the config file (if present) and applies FUJIN_* env overrides.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = DefaultPath()
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if v := os.Getenv("FUJIN_PRIMARY_URL"); v != "" {
		cfg.Primary.URL = v
	}
	if v := os.Getenv("FUJIN_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("FUJIN_TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.TelegramBotToken = v
	}
	if v := os.Getenv("FUJIN_TELEGRAM_CHAT_ID"); v != "" {
		cfg.TelegramChatID = v
	}

	if cfg.Primary.URL == "" {
		return nil, fmt.Errorf("config: primary remote URL is required (set it in %s or FUJIN_PRIMARY_URL)", path)
	}
	return &cfg, nil
}

// AllRemotes returns primary + failovers in order.
func (c *Config) AllRemotes() []Remote {
	remotes := []Remote{c.Primary}
	return append(remotes, c.Failover...)
}
