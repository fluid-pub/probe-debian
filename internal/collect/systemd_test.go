package collect

import "testing"

func TestParseEnabledServiceUnits(t *testing.T) {
	raw := `UNIT FILE                              STATE   PRESET
apparmor.service                         enabled enabled
nginx.service                            enabled enabled
`
	got := parseEnabledServiceUnits([]byte(raw))
	if len(got) != 2 {
		t.Fatalf("expected 2 units, got %d: %v", len(got), got)
	}
	if got[0] != "apparmor.service" || got[1] != "nginx.service" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestParseSystemctlShowToMap(t *testing.T) {
	raw := `Id=nginx.service
ActiveState=active
SubState=running
UnitFileState=enabled
LoadState=loaded

Id=ssh.service
ActiveState=inactive
SubState=dead
UnitFileState=enabled
LoadState=loaded
`
	m := parseSystemctlShowToMap([]byte(raw))
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	ng := m["nginx.service"]
	if ng["ActiveState"] != "active" || ng["SubState"] != "running" {
		t.Fatalf("nginx map: %#v", ng)
	}
}
