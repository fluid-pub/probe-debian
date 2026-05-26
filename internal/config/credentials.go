package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type credentialsFile struct {
	Auth         AuthConfig         `yaml:"auth"`
	Controlplane ControlplaneConfig `yaml:"controlplane"`
}

// MergeCredentialsFromFile overlays auth and controlplane from a credentials YAML (0600 on write).
// If the file does not exist, returns nil.
func MergeCredentialsFromFile(cfg *Config, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read credentials file: %w", err)
	}
	var f credentialsFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("decode credentials file: %w", err)
	}
	if strings.TrimSpace(f.Auth.OrganizationUUID) != "" {
		cfg.Auth.OrganizationUUID = strings.TrimSpace(f.Auth.OrganizationUUID)
	}
	if strings.TrimSpace(f.Auth.Token) != "" {
		cfg.Auth.Token = strings.TrimSpace(f.Auth.Token)
	}
	if strings.TrimSpace(f.Controlplane.BaseURL) != "" {
		cfg.Controlplane.BaseURL = strings.TrimSpace(f.Controlplane.BaseURL)
	}
	return nil
}

// WriteCredentialsFile writes durable probe HTTP credentials (0600, atomic replace).
func WriteCredentialsFile(path string, cfg *Config) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("credentials path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir credentials dir: %w", err)
	}
	out, err := yaml.Marshal(&credentialsFile{
		Auth: AuthConfig{
			OrganizationUUID: strings.TrimSpace(cfg.Auth.OrganizationUUID),
			Token:            strings.TrimSpace(cfg.Auth.Token),
		},
		Controlplane: ControlplaneConfig{
			BaseURL: strings.TrimSpace(cfg.Controlplane.BaseURL),
		},
	})
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".fluid-probe-credentials-*")
	if err != nil {
		return fmt.Errorf("create temp credentials: %w", err)
	}
	tmpName := tmp.Name()
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("sync temp credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp credentials: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace credentials file: %w", err)
	}
	return nil
}
