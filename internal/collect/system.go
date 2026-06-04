package collect

import (
	"context"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemSnapshot struct {
	// ID is the stable host key (host_id or hostname) for control plane entity diff; set before push.
	ID            string        `json:"id,omitempty"`
	CPU           CPUInfo       `json:"cpu"`
	Memory        MemoryInfo    `json:"memory"`
	Disks         []DiskMetric  `json:"disks"`
	OsMaintenance OsMaintenance `json:"os_maintenance"`
}

type CPUInfo struct {
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	UsedPct   float64 `json:"used_percent"`
}

type DiskMetric struct {
	Mountpoint string  `json:"mountpoint"`
	Fstype     string  `json:"fstype"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	UsedPct    float64 `json:"used_percent"`
	InodesUsed uint64  `json:"inodes_used"`
	InodesFree uint64  `json:"inodes_free"`
}

func CollectSystem(_ context.Context) (*SystemSnapshot, error) {
	cpuPct, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	avg, err := load.Avg()
	if err != nil {
		return nil, err
	}

	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	disks := make([]DiskMetric, 0, len(parts))
	for _, p := range parts {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		disks = append(disks, DiskMetric{
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Total:      usage.Total,
			Used:       usage.Used,
			Free:       usage.Free,
			UsedPct:    usage.UsedPercent,
			InodesUsed: usage.InodesUsed,
			InodesFree: usage.InodesFree,
		})
	}

	usage := 0.0
	if len(cpuPct) > 0 {
		usage = cpuPct[0]
	}
	return &SystemSnapshot{
		CPU: CPUInfo{
			UsagePercent: usage,
			Load1:        avg.Load1,
			Load5:        avg.Load5,
			Load15:       avg.Load15,
		},
		Memory: MemoryInfo{
			Total:     vm.Total,
			Used:      vm.Used,
			Free:      vm.Free,
			Available: vm.Available,
			UsedPct:   vm.UsedPercent,
		},
		Disks:         disks,
		OsMaintenance: collectOsMaintenance(defaultRebootRequiredPath, defaultRebootPkgsPath, defaultMaxRebootPkgLines),
	}, nil
}
