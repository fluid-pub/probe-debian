package collect

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

type PackageUpdate struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installed_version"`
	CandidateVersion string `json:"candidate_version"`
}

type PackageSnapshot struct {
	Manager       string          `json:"manager"`
	Supported     bool            `json:"supported"`
	TotalUpgrades int             `json:"total_upgrades"`
	Items         []PackageUpdate `json:"items"`
	Error         string          `json:"error,omitempty"`
}

func CollectAPT(ctx context.Context, maxItems int) PackageSnapshot {
	if _, err := exec.LookPath("apt"); err != nil {
		return PackageSnapshot{Manager: "apt", Supported: false, Error: "apt not found"}
	}

	cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return PackageSnapshot{Manager: "apt", Supported: true, Error: err.Error()}
	}

	res := PackageSnapshot{
		Manager:   "apt",
		Supported: true,
		Items:     make([]PackageUpdate, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		name, installed, candidate, ok := parseUpgradableLine(line)
		if !ok {
			continue
		}
		res.TotalUpgrades++
		if len(res.Items) < maxItems {
			res.Items = append(res.Items, PackageUpdate{
				Name:             name,
				InstalledVersion: installed,
				CandidateVersion: candidate,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		res.Error = err.Error()
	}
	return res
}

func parseUpgradableLine(line string) (name, installed, candidate string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "Listing...") {
		return "", "", "", false
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", "", false
	}
	pkgAndChannel := parts[0]
	name = strings.Split(pkgAndChannel, "/")[0]
	candidate = strings.TrimSpace(parts[1])

	idx := strings.Index(line, "[upgradable from:")
	if idx == -1 {
		return "", "", "", false
	}
	installedPart := line[idx+len("[upgradable from:"):]
	installed = strings.TrimSpace(strings.TrimSuffix(installedPart, "]"))
	if name == "" || installed == "" || candidate == "" {
		return "", "", "", false
	}
	return name, installed, candidate, true
}
