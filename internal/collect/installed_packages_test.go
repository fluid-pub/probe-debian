package collect

import "testing"

func TestCollectInstalledPackages_idShape(t *testing.T) {
	p := InstalledPackage{Name: "bash", Version: "5.0", Architecture: "amd64"}
	p.ID = p.Name + ":" + p.Architecture
	if p.ID != "bash:amd64" {
		t.Fatalf("unexpected id: %s", p.ID)
	}
}
