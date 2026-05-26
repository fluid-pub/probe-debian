package collect

import "testing"

func TestParseUpgradableLine(t *testing.T) {
	line := "bash/stable 5.2.15-2+b7 amd64 [upgradable from: 5.2.15-2+b2]"
	name, installed, candidate, ok := parseUpgradableLine(line)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if name != "bash" {
		t.Fatalf("unexpected name: %s", name)
	}
	if installed != "5.2.15-2+b2" {
		t.Fatalf("unexpected installed: %s", installed)
	}
	if candidate != "5.2.15-2+b7" {
		t.Fatalf("unexpected candidate: %s", candidate)
	}
}
