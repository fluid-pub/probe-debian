package config

import (
	"testing"

	"fluid/probes/core"
)

func TestApplyEntityIntervalsToCollection(t *testing.T) {
	c := CollectionConfig{}
	entities := []core.EntityConfig{
		{Name: entitySystemMetrics, RefreshInterval: "10s"},
		{Name: entityFileChecks, RefreshInterval: "60s"},
		{Name: entityPackageUpdates, RefreshInterval: "30m"},
	}
	applyEntityIntervalsToCollection(&c, entities)
	if c.SystemInterval != "10s" || c.FilesInterval != "60s" || c.APTInterval != "30m" {
		t.Fatalf("collection: %+v", c)
	}
}

func TestApplyRuntimeOverlay_entitiesMapToCollection(t *testing.T) {
	local := &Config{
		Probe: ProbeConfig{Name: "p1"},
		Collection: CollectionConfig{
			SystemInterval:            "30s",
			FilesInterval:             "60s",
			APTInterval:               "30m",
			InstalledPackagesInterval: "1h",
			ServicesInterval:          "30m",
			RequestTimeout:            "10s",
		},
	}
	overlay, _, err := ParseRuntimeResponse([]byte(`{
		"runtime_config": {
			"data": {
				"entities": [
					{"name": "debian_system_metrics", "refresh_interval": "15s"},
					{"name": "debian_file_checks", "refresh_interval": "45s"}
				]
			}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := ApplyRuntimeOverlay(local, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Collection.SystemInterval != "15s" || merged.Collection.FilesInterval != "45s" {
		t.Fatalf("collection: %+v", merged.Collection)
	}
}
