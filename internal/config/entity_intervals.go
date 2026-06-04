package config

import "fluid/probes/core"

// Entity names emitted by the Debian probe; used to map data.entities refresh_interval
// onto collection intervals when runtime_config.collection is absent.
const (
	entitySystemMetrics     = "debian_system_metrics"
	entityFilesystem        = "debian_filesystem"
	entityFileChecks        = "debian_file_checks"
	entityPackageUpdates    = "debian_package_updates"
	entityInstalledPackages = "debian_installed_packages"
	entitySystemdServices   = "debian_systemd_services"
)

func applyEntityIntervalsToCollection(c *CollectionConfig, entities []core.EntityConfig) {
	if c == nil {
		return
	}
	for _, e := range entities {
		if e.Name == "" || e.RefreshInterval == "" {
			continue
		}
		switch e.Name {
		case entitySystemMetrics, entityFilesystem:
			c.SystemInterval = e.RefreshInterval
		case entityFileChecks:
			c.FilesInterval = e.RefreshInterval
		case entityPackageUpdates:
			c.APTInterval = e.RefreshInterval
		case entityInstalledPackages:
			c.InstalledPackagesInterval = e.RefreshInterval
		case entitySystemdServices:
			c.ServicesInterval = e.RefreshInterval
		}
	}
}
