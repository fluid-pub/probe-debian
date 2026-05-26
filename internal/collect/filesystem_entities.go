package collect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// FilesystemEntity is one row per mountpoint for state queries (flat, queryable).
type FilesystemEntity struct {
	ID               string  `json:"id"`
	Fluid            string  `json:"fluid"`
	Hostname         string  `json:"hostname"`
	Mountpoint       string  `json:"mountpoint"`
	Fstype           string  `json:"fstype"`
	Total            uint64  `json:"total"`
	Used             uint64  `json:"used"`
	Free             uint64  `json:"free"`
	UsedPercent      float64 `json:"used_percent"`
	AvailablePercent float64 `json:"available_percent"`
}

// FilesystemEntities builds queryable filesystem rows from a system snapshot.
func FilesystemEntities(snap *SystemSnapshot, hostname string) []FilesystemEntity {
	if snap == nil || len(snap.Disks) == 0 {
		return nil
	}
	out := make([]FilesystemEntity, 0, len(snap.Disks))
	for _, d := range snap.Disks {
		id := fmt.Sprintf("%s:%s", hostname, d.Mountpoint)
		fluid := sha256.Sum256([]byte(id))
		avail := 100.0 - d.UsedPct
		if avail < 0 {
			avail = 0
		}
		out = append(out, FilesystemEntity{
			ID:               id,
			Fluid:            hex.EncodeToString(fluid[:]),
			Hostname:         hostname,
			Mountpoint:       d.Mountpoint,
			Fstype:           d.Fstype,
			Total:            d.Total,
			Used:             d.Used,
			Free:             d.Free,
			UsedPercent:      d.UsedPct,
			AvailablePercent: avail,
		})
	}
	return out
}
