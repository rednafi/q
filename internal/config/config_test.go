package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_NotExist(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.DefaultModel != "" {
		t.Errorf("expected DefaultModel empty, got %q", cfg.DefaultModel)
	}
	if len(cfg.APIKeys) != 0 {
		t.Errorf("expected no APIKeys, got %v", cfg.APIKeys)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	wantModel := "a/b"
	wantKey := "key123"
	cfg := Config{DefaultModel: wantModel, APIKeys: map[string]string{"openai": wantKey}}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if got.DefaultModel != wantModel {
		t.Errorf("DefaultModel = %q; want %q", got.DefaultModel, wantModel)
	}
	if v := got.APIKeys["openai"]; v != wantKey {
		t.Errorf("APIKeys[openai] = %q; want %q", v, wantKey)
	}
}

func TestConfigPath_Fallback(t *testing.T) {
	// Unset XDG_CONFIG_HOME to use UserConfigDir fallback
	os.Unsetenv("XDG_CONFIG_HOME")
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath fallback error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("q", "config.json")) {
		t.Errorf("ConfigPath fallback = %q; want suffix %q", path, filepath.Join("q", "config.json"))
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	if err := os.WriteFile(path, []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Error("expected error loading invalid JSON, got nil")
	}
}

func TestSetAndGetAPIKey(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	key := "testkey123"
	if err := SetAPIKey("foo", key); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	got, err := GetAPIKey("foo")
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if got != key {
		t.Errorf("expected APIKey %q, got %q", key, got)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), key) {
		t.Errorf("config file %s does not contain key %q: %s", path, key, string(data))
	}
}

func TestSetAndGetDefaultModel(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	model := "provider/model"
	if err := SetDefaultModel(model); err != nil {
		t.Fatalf("SetDefaultModel: %v", err)
	}
	got, err := GetDefaultModel()
	if err != nil {
		t.Fatalf("GetDefaultModel: %v", err)
	}
	if got != model {
		t.Errorf("expected default model %q, got %q", model, got)
	}
}

func TestConfigPath(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	want := filepath.Join(tmp, "q", "config.json")
	if path != want {
		t.Errorf("expected ConfigPath %q, got %q", want, path)
	}
}
