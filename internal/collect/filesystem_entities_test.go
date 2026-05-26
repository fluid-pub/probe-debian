package collect

import "testing"

func TestFilesystemEntitiesAvailablePercent(t *testing.T) {
	snap := &SystemSnapshot{
		Disks: []DiskMetric{
			{Mountpoint: "/", Fstype: "ext4", UsedPct: 85},
		},
	}
	rows := FilesystemEntities(snap, "host1")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].AvailablePercent != 15 {
		t.Fatalf("expected 15%% available, got %v", rows[0].AvailablePercent)
	}
	if rows[0].ID != "host1:/" {
		t.Fatalf("unexpected id %q", rows[0].ID)
	}
}
