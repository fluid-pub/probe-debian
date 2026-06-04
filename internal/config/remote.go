package config

// ApplyRuntimeConfig merges control plane runtime_config onto local config.
// Prefer ParseRuntimeResponse + ApplyRuntimeOverlay for the full Debian payload.
func ApplyRuntimeConfig(local *Config, remote *RuntimeOverlay) (*Config, error) {
	return ApplyRuntimeOverlay(local, remote)
}
