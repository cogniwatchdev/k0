// Package config handles loading and persisting K-0 user configuration.
// Config is stored at ~/.kiai/config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultOllamaAddr is the address of the bundled Ollama instance.
	DefaultOllamaAddr = "http://127.0.0.1:11434"
	// DefaultModel is the primary text model served by Ollama.
	DefaultModel = "k0-pentest:latest"
	// DefaultGatewayAddr is the internal gateway address.
	DefaultGatewayAddr = "http://127.0.0.1:19876"
	// ConfigDir is the user config directory name.
	ConfigDir = ".kiai"
	// ConfigFile is the config filename within ConfigDir.
	ConfigFile = "config.json"
)

// Config holds all user-configurable settings for K-0.
type Config struct {
	// LLM settings
	OllamaAddr     string `json:"ollama_addr"`
	Model          string `json:"model"`
	EmbeddingModel string `json:"embedding_model,omitempty"`

	// Memory settings
	MemoryPath      string `json:"memory_path"`
	SemanticMemory  bool   `json:"semantic_memory"`
	SummarizeEvery  int    `json:"summarize_every_mins"`

	// Gateway
	GatewayAddr string `json:"gateway_addr"`

	// Recon settings
	WebSearchEnabled bool   `json:"web_search_enabled"`
	WebSearchProvider string `json:"web_search_provider,omitempty"`

	// Scope
	ScopeFile string `json:"scope_file,omitempty"`

	// Telemetry: always false, present for schema clarity
	Telemetry bool `json:"telemetry"`

	// TUI preferences
	Theme string `json:"theme"` // "kali-purple" (default)
}

// Defaults returns a Config populated with safe, offline-first defaults.
func Defaults() *Config {
	home, _ := os.UserHomeDir()
	memPath := filepath.Join(home, ConfigDir, "memory")
	return &Config{
		OllamaAddr:     DefaultOllamaAddr,
		Model:          DefaultModel,
		MemoryPath:     memPath,
		SemanticMemory: false,
		SummarizeEvery: 20,
		GatewayAddr:    DefaultGatewayAddr,
		WebSearchEnabled: false,
		Telemetry:      false,
		Theme:          "kali-purple",
	}
}

// Load reads the config from disk, falling back to Defaults if not found.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Defaults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg := Defaults()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// Save persists the config to disk, creating directories as needed.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// MemoryDir returns the absolute path for a named memory subdirectory.
// e.g. MemoryDir(cfg, "episodes") → ~/.kiai/memory/episodes
func MemoryDir(cfg *Config, sub string) string {
	return filepath.Join(cfg.MemoryPath, sub)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ConfigDir, ConfigFile), nil
}
