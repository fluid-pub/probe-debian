package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Probe        ProbeConfig        `yaml:"probe"`
	Auth         AuthConfig         `yaml:"auth"`
	Controlplane ControlplaneConfig `yaml:"controlplane"`
	Collection   CollectionConfig   `yaml:"collection"`
	Spool        SpoolConfig        `yaml:"spool"`
	Files        []FileRule         `yaml:"files"`
	Directories  []DirectoryRule    `yaml:"directories"`
}

type ProbeConfig struct {
	Name     string `yaml:"name"`
	HostID   string `yaml:"host_id"`
	Hostname string `yaml:"hostname"`
	Version  string `yaml:"version"`
}

type AuthConfig struct {
	OrganizationUUID string `yaml:"organization_uuid"`
	Token            string `yaml:"token"`
}

type ControlplaneConfig struct {
	BaseURL string `yaml:"base_url"`
}

type CollectionConfig struct {
	SystemInterval            string `yaml:"system_interval"`
	FilesInterval             string `yaml:"files_interval"`
	APTInterval               string `yaml:"apt_interval"`
	InstalledPackagesInterval string `yaml:"installed_packages_interval"`
	ServicesInterval          string `yaml:"services_interval"`
	RequestTimeout            string `yaml:"request_timeout"`
	MaxPackageItems           int    `yaml:"max_package_items"`
	MaxHashFileSize           int64  `yaml:"max_hash_file_size_bytes"`
}

type SpoolConfig struct {
	Path       string `yaml:"path"`
	MaxEntries int    `yaml:"max_entries"`
}

type FileRule struct {
	Path string `yaml:"path"`
}

type DirectoryRule struct {
	Path      string `yaml:"path"`
	Recursive bool   `yaml:"recursive"`
}

type Runtime struct {
	Raw Config

	SystemInterval            time.Duration
	FilesInterval             time.Duration
	APTInterval               time.Duration
	InstalledPackagesInterval time.Duration
	ServicesInterval          time.Duration
	RequestTimeout            time.Duration
}

// ParseConfigFile reads YAML, applies env substitution and defaults. Does not validate.
func ParseConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	resolveEnv(&cfg)
	applyDefaults(&cfg)
	return &cfg, nil
}

// NewRuntime validates config and builds a Runtime with parsed durations.
func NewRuntime(cfg *Config) (*Runtime, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}

	systemInterval, _ := time.ParseDuration(cfg.Collection.SystemInterval)
	filesInterval, _ := time.ParseDuration(cfg.Collection.FilesInterval)
	aptInterval, _ := time.ParseDuration(cfg.Collection.APTInterval)
	installedPkgsInterval, _ := time.ParseDuration(cfg.Collection.InstalledPackagesInterval)
	servicesInterval, _ := time.ParseDuration(cfg.Collection.ServicesInterval)
	requestTimeout, _ := time.ParseDuration(cfg.Collection.RequestTimeout)

	return &Runtime{
		Raw:                       *cfg,
		SystemInterval:            systemInterval,
		FilesInterval:             filesInterval,
		APTInterval:               aptInterval,
		InstalledPackagesInterval: installedPkgsInterval,
		ServicesInterval:          servicesInterval,
		RequestTimeout:            requestTimeout,
	}, nil
}

// Load reads a complete valid configuration from a single file.
func Load(path string) (*Runtime, error) {
	cfg, err := ParseConfigFile(path)
	if err != nil {
		return nil, err
	}
	return NewRuntime(cfg)
}

func applyDefaults(cfg *Config) {
	if cfg.Probe.Version == "" {
		cfg.Probe.Version = "0.1.0"
	}
	if cfg.Probe.Hostname == "" {
		if host, err := os.Hostname(); err == nil {
			cfg.Probe.Hostname = host
		}
	}
	if cfg.Collection.SystemInterval == "" {
		cfg.Collection.SystemInterval = "30s"
	}
	if cfg.Collection.FilesInterval == "" {
		cfg.Collection.FilesInterval = "60s"
	}
	if cfg.Collection.APTInterval == "" {
		cfg.Collection.APTInterval = "30m"
	}
	if cfg.Collection.InstalledPackagesInterval == "" {
		cfg.Collection.InstalledPackagesInterval = "1h"
	}
	if cfg.Collection.ServicesInterval == "" {
		cfg.Collection.ServicesInterval = "30m"
	}
	if cfg.Collection.RequestTimeout == "" {
		cfg.Collection.RequestTimeout = "10s"
	}
	if cfg.Collection.MaxPackageItems <= 0 {
		cfg.Collection.MaxPackageItems = 200
	}
	if cfg.Collection.MaxHashFileSize <= 0 {
		cfg.Collection.MaxHashFileSize = 5 * 1024 * 1024
	}
	if cfg.Spool.Path == "" {
		cfg.Spool.Path = "/var/lib/fluid-probe/spool.jsonl"
	}
	if cfg.Spool.MaxEntries <= 0 {
		cfg.Spool.MaxEntries = 2000
	}
}

func validate(cfg *Config) error {
	if cfg.Probe.Name == "" {
		return fmt.Errorf("probe.name is required")
	}
	if cfg.Auth.OrganizationUUID == "" || cfg.Auth.Token == "" {
		return fmt.Errorf("auth.organization_uuid and auth.token are required")
	}
	if cfg.Controlplane.BaseURL == "" {
		return fmt.Errorf("controlplane.base_url is required")
	}
	if _, err := time.ParseDuration(cfg.Collection.SystemInterval); err != nil {
		return fmt.Errorf("invalid collection.system_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Collection.FilesInterval); err != nil {
		return fmt.Errorf("invalid collection.files_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Collection.APTInterval); err != nil {
		return fmt.Errorf("invalid collection.apt_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Collection.InstalledPackagesInterval); err != nil {
		return fmt.Errorf("invalid collection.installed_packages_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Collection.ServicesInterval); err != nil {
		return fmt.Errorf("invalid collection.services_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Collection.RequestTimeout); err != nil {
		return fmt.Errorf("invalid collection.request_timeout: %w", err)
	}

	dir := filepath.Dir(cfg.Spool.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create spool directory: %w", err)
	}
	return nil
}

func resolveEnv(cfg *Config) {
	cfg.Probe.Name = resolve(cfg.Probe.Name)
	cfg.Probe.HostID = resolve(cfg.Probe.HostID)
	cfg.Probe.Hostname = resolve(cfg.Probe.Hostname)
	cfg.Probe.Version = resolve(cfg.Probe.Version)
	cfg.Auth.OrganizationUUID = resolve(cfg.Auth.OrganizationUUID)
	cfg.Auth.Token = resolve(cfg.Auth.Token)
	cfg.Controlplane.BaseURL = resolve(cfg.Controlplane.BaseURL)
	cfg.Spool.Path = resolve(cfg.Spool.Path)

	for i := range cfg.Files {
		cfg.Files[i].Path = resolve(cfg.Files[i].Path)
	}
	for i := range cfg.Directories {
		cfg.Directories[i].Path = resolve(cfg.Directories[i].Path)
	}
}

func resolve(v string) string {
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return os.Getenv(strings.TrimSuffix(strings.TrimPrefix(v, "${"), "}"))
	}
	return v
}
