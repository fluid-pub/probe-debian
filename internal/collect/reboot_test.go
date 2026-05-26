package collect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectOsMaintenance_noMarker(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "missing")
	pkgs := filepath.Join(dir, "pkgs")
	got := collectOsMaintenance(marker, pkgs, 10)
	if got.RebootRequired {
		t.Fatalf("expected reboot_required false")
	}
	if len(got.RebootRequiredPackages) != 0 {
		t.Fatalf("expected no packages")
	}
}

func TestCollectOsMaintenance_withMarkerAndPkgs(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "reboot-required")
	if err := os.WriteFile(marker, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := filepath.Join(dir, "pkgs")
	content := "linux-image-amd64\nlibc6\n"
	if err := os.WriteFile(pkgs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collectOsMaintenance(marker, pkgs, 10)
	if !got.RebootRequired {
		t.Fatalf("expected reboot_required true")
	}
	if len(got.RebootRequiredPackages) != 2 {
		t.Fatalf("packages: got %d want 2: %v", len(got.RebootRequiredPackages), got.RebootRequiredPackages)
	}
	if got.RebootPackagesTruncated {
		t.Fatalf("unexpected truncated")
	}
}

func TestCollectOsMaintenance_truncatesPackages(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "reboot-required")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := filepath.Join(dir, "pkgs")
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("pkg")
		b.WriteByte(byte('0' + i))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(pkgs, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collectOsMaintenance(marker, pkgs, 3)
	if !got.RebootRequired {
		t.Fatal("expected reboot required")
	}
	if len(got.RebootRequiredPackages) != 3 {
		t.Fatalf("want 3 packages, got %d", len(got.RebootRequiredPackages))
	}
	if !got.RebootPackagesTruncated {
		t.Fatal("expected truncated flag")
	}
}
