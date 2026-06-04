package config

import "testing"

func TestApplyRuntimeOverlay_collectionAndFiles(t *testing.T) {
	local := &Config{
		Probe: ProbeConfig{Name: "p1"},
		Collection: CollectionConfig{
			SystemInterval:            "30s",
			FilesInterval:             "60s",
			APTInterval:               "30m",
			InstalledPackagesInterval: "1h",
			ServicesInterval:          "30m",
			RequestTimeout:            "10s",
			MaxPackageItems:           200,
			MaxHashFileSize:           5242880,
		},
		Files: []FileRule{{Path: "/etc/fstab"}},
	}
	overlay, _, err := ParseRuntimeResponse([]byte(`{
		"runtime_config": {
			"collection": {"system_interval": "10s"},
			"files": [{"path": "/etc/hosts"}, {"path": "/etc/fstab"}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := ApplyRuntimeOverlay(local, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Collection.SystemInterval != "10s" {
		t.Fatalf("system_interval: %q", merged.Collection.SystemInterval)
	}
	if len(merged.Files) != 2 {
		t.Fatalf("files: %+v", merged.Files)
	}
}
