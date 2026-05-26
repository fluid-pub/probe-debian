package collect

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

const (
	defaultRebootRequiredPath = "/var/run/reboot-required"
	defaultRebootPkgsPath     = "/var/run/reboot-required.pkgs"
	defaultMaxRebootPkgLines  = 200
)

// OsMaintenance reports OS-level maintenance signals (Debian/Ubuntu reboot-required convention).
type OsMaintenance struct {
	RebootRequired          bool     `json:"reboot_required"`
	RebootRequiredPackages  []string `json:"reboot_required_packages,omitempty"`
	RebootPackagesTruncated bool     `json:"reboot_required_packages_truncated,omitempty"`
}

// collectOsMaintenance reads reboot-required markers from the given paths (for tests, pass temp files).
func collectOsMaintenance(rebootMarkerPath, pkgsPath string, maxPkgLines int) OsMaintenance {
	if maxPkgLines <= 0 {
		maxPkgLines = defaultMaxRebootPkgLines
	}
	out := OsMaintenance{}
	if rebootMarkerPath == "" {
		rebootMarkerPath = defaultRebootRequiredPath
	}
	if pkgsPath == "" {
		pkgsPath = defaultRebootPkgsPath
	}

	if _, err := os.Stat(rebootMarkerPath); err != nil {
		return out
	}
	out.RebootRequired = true

	data, err := os.ReadFile(pkgsPath)
	if err != nil || len(data) == 0 {
		return out
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if len(out.RebootRequiredPackages) >= maxPkgLines {
			out.RebootPackagesTruncated = true
			break
		}
		out.RebootRequiredPackages = append(out.RebootRequiredPackages, line)
	}
	return out
}
