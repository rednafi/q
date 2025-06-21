package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the unified configuration payload stored at $XDG_CONFIG_HOME/q/config.json.
// It contains the default model and API keys for all providers.
type Config struct {
	Comment      string            `json:"// Note,omitempty"`
	DefaultModel string            `json:"default_model"`
	APIKeys      map[string]string `json:"api_keys"`
}

const configFileName = "config.json"

// configDir returns the XDG config dir for the app.
func configDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "q"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "q"), nil
}

// configPath returns the full path to the config file.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// LoadConfig loads the configuration, returning defaults if missing.
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{APIKeys: make(map[string]string)}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = make(map[string]string)
	}
	return cfg, nil
}

// SaveConfig persists the configuration to disk.
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Add warning comment about API credentials
	cfg.Comment = "This file stores secret API credentials. Do not share!"

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// GetAPIKey returns the API key for a provider, or empty if not set.
func GetAPIKey(provider string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.APIKeys[provider], nil
}

// SetAPIKey sets and persists an API key for a provider.
func SetAPIKey(provider, key string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.APIKeys[provider] = key
	return SaveConfig(cfg)
}

// GetDefaultModel returns the stored default model identifier.
func GetDefaultModel() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.DefaultModel, nil
}

// SetDefaultModel sets and persists the default model identifier.
func SetDefaultModel(model string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.DefaultModel = model
	return SaveConfig(cfg)
}

// ConfigPath returns the full filesystem path to the config file (config.json).
func ConfigPath() (string, error) {
	return configPath()
}
