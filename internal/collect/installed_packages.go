package collect

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// InstalledPackage is one row for entity type debian_installed_packages.
// id must be stable across runs: name + ":" + architecture.
type InstalledPackage struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
}

// CollectInstalledPackages lists all installed packages via dpkg-query (Debian/Ubuntu).
func CollectInstalledPackages(ctx context.Context) ([]InstalledPackage, error) {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return nil, err
	}

	// Machine-readable: package, version, architecture per line.
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f", "${Package}\t${Version}\t${Architecture}\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rows []InstalledPackage
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		name, version, arch := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		if name == "" || arch == "" {
			continue
		}
		rows = append(rows, InstalledPackage{
			ID:           name + ":" + arch,
			Name:         name,
			Version:      version,
			Architecture: arch,
		})
	}
	if err := sc.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}
