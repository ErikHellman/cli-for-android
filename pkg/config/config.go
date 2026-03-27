// Package config manages acli's persistent configuration via viper.
// Config file location: ~/.acli/config.yaml
// All settings are also readable from environment variables prefixed ACLI_.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const envPrefix = "ACLI"

// Config holds the acli configuration schema.
type Config struct {
	// DefaultDevice is the ADB serial to target when --device is not specified.
	DefaultDevice string `mapstructure:"default_device"`

	// SDKRoot overrides Android SDK root discovery.
	SDKRoot string `mapstructure:"sdk_root"`

	// GithubRepo is the repo used for self-update checks (owner/repo).
	GithubRepo string `mapstructure:"github_repo"`
}

var (
	v      *viper.Viper
	loaded Config
)

// Load reads the config file and environment variables.
// Safe to call multiple times; subsequent calls are no-ops.
func Load() error {
	if v != nil {
		return nil
	}
	v = viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("github_repo", "android-cli/acli")

	cfgDir, err := configDir()
	if err != nil {
		return err
	}
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(cfgDir)

	if err := v.ReadInConfig(); err != nil {
		// Not an error if the file simply doesn't exist yet
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	return v.Unmarshal(&loaded)
}

// Get returns the loaded Config. Load() must have been called first.
func Get() Config { return loaded }

// Set updates a single key in memory and persists the config file.
func Set(key, value string) error {
	if v == nil {
		if err := Load(); err != nil {
			return err
		}
	}
	v.Set(key, value)
	return save()
}

// ── helpers ───────────────────────────────────────────────────────────────

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".acli")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	return dir, nil
}

func save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	return v.WriteConfigAs(filepath.Join(dir, "config.yaml"))
}
