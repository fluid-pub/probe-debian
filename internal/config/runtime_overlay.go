package config

import (
	"encoding/json"
	"fmt"
	"time"

	"fluid/probes/core"
)

// RuntimeOverlay is the Debian-specific subset of control plane runtime_config.
type RuntimeOverlay struct {
	Data *struct {
		Entities []core.EntityConfig `json:"entities"`
	} `json:"data,omitempty"`
	Collection  *CollectionConfig `json:"collection,omitempty"`
	Files       []FileRule        `json:"files,omitempty"`
	Directories []DirectoryRule   `json:"directories,omitempty"`
}

// ParseRuntimeResponse parses GET /probes/config JSON into overlay + config_version.
func ParseRuntimeResponse(data []byte) (*RuntimeOverlay, string, error) {
	var raw struct {
		RuntimeConfig *RuntimeOverlay `json:"runtime_config"`
		ConfigVersion string          `json:"config_version"`
		Data          *struct {
			Entities []core.EntityConfig `json:"entities"`
		} `json:"data"`
		Collection  *CollectionConfig `json:"collection,omitempty"`
		Files       []FileRule        `json:"files,omitempty"`
		Directories []DirectoryRule   `json:"directories,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, "", fmt.Errorf("parse runtime response: %w", err)
	}
	if raw.RuntimeConfig != nil {
		return raw.RuntimeConfig, raw.ConfigVersion, nil
	}
	if raw.Data != nil || raw.Collection != nil || len(raw.Files) > 0 || len(raw.Directories) > 0 {
		return &RuntimeOverlay{
			Data:        raw.Data,
			Collection:  raw.Collection,
			Files:       raw.Files,
			Directories: raw.Directories,
		}, raw.ConfigVersion, nil
	}
	return nil, raw.ConfigVersion, nil
}

// ApplyRuntimeOverlay merges control plane runtime_config onto a copy of local Config.
func ApplyRuntimeOverlay(local *Config, remote *RuntimeOverlay) (*Config, error) {
	if local == nil {
		return nil, fmt.Errorf("local config is nil")
	}
	if remote == nil {
		return local, nil
	}

	out := *local
	mergedCollection := out.Collection

	if remote.Data != nil && len(remote.Data.Entities) > 0 {
		fromEntities := CollectionConfig{}
		applyEntityIntervalsToCollection(&fromEntities, remote.Data.Entities)
		mergedCollection = mergeCollectionConfig(mergedCollection, fromEntities)
	}
	if remote.Collection != nil {
		mergedCollection = mergeCollectionConfig(mergedCollection, *remote.Collection)
	}
	if err := validateCollectionDurations(mergedCollection); err != nil {
		return nil, err
	}
	out.Collection = mergedCollection

	if len(remote.Files) > 0 {
		out.Files = append([]FileRule(nil), remote.Files...)
	}
	if len(remote.Directories) > 0 {
		out.Directories = append([]DirectoryRule(nil), remote.Directories...)
	}

	return &out, nil
}

func mergeCollectionConfig(local, remote CollectionConfig) CollectionConfig {
	out := local
	if remote.SystemInterval != "" {
		out.SystemInterval = remote.SystemInterval
	}
	if remote.FilesInterval != "" {
		out.FilesInterval = remote.FilesInterval
	}
	if remote.APTInterval != "" {
		out.APTInterval = remote.APTInterval
	}
	if remote.InstalledPackagesInterval != "" {
		out.InstalledPackagesInterval = remote.InstalledPackagesInterval
	}
	if remote.ServicesInterval != "" {
		out.ServicesInterval = remote.ServicesInterval
	}
	if remote.RequestTimeout != "" {
		out.RequestTimeout = remote.RequestTimeout
	}
	if remote.MaxPackageItems > 0 {
		out.MaxPackageItems = remote.MaxPackageItems
	}
	if remote.MaxHashFileSize > 0 {
		out.MaxHashFileSize = remote.MaxHashFileSize
	}
	return out
}

func validateCollectionDurations(c CollectionConfig) error {
	for _, d := range []struct {
		name string
		val  string
	}{
		{"system_interval", c.SystemInterval},
		{"files_interval", c.FilesInterval},
		{"apt_interval", c.APTInterval},
		{"installed_packages_interval", c.InstalledPackagesInterval},
		{"services_interval", c.ServicesInterval},
		{"request_timeout", c.RequestTimeout},
	} {
		if _, err := time.ParseDuration(d.val); err != nil {
			return fmt.Errorf("invalid collection.%s: %w", d.name, err)
		}
	}
	return nil
}
