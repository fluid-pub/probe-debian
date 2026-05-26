package collect

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

const systemctlShowChunk = 64

// SystemdService is one row for entity type debian_systemd_services (enabled unit files + runtime state).
type SystemdService struct {
	ID            string `json:"id"`
	Unit          string `json:"unit"`
	UnitFileState string `json:"unit_file_state"`
	ActiveState   string `json:"active_state"`
	SubState      string `json:"sub_state"`
	LoadState     string `json:"load_state"`
}

// CollectEnabledServices lists enabled service units and enriches them with runtime properties from systemctl show.
func CollectEnabledServices(ctx context.Context) ([]SystemdService, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, err
	}

	// enabled-runtime covers units enabled for the current boot session.
	listCmd := exec.CommandContext(ctx, "systemctl", "list-unit-files",
		"--type=service", "--state=enabled", "--state=enabled-runtime", "--no-pager", "--no-legend")
	listOut, err := listCmd.Output()
	if err != nil {
		return nil, err
	}

	units := parseEnabledServiceUnits(listOut)
	if len(units) == 0 {
		return nil, nil
	}

	var all []SystemdService
	for start := 0; start < len(units); start += systemctlShowChunk {
		end := start + systemctlShowChunk
		if end > len(units) {
			end = len(units)
		}
		chunk := units[start:end]
		states, err := systemctlShowStates(ctx, chunk)
		if err != nil {
			return all, err
		}
		all = append(all, states...)
	}
	return all, nil
}

func parseEnabledServiceUnits(data []byte) []string {
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		unit := fields[0]
		if !strings.HasSuffix(unit, ".service") {
			continue
		}
		// Skip header row if present (e.g. "UNIT FILE").
		if strings.EqualFold(unit, "UNIT") || strings.HasPrefix(strings.ToUpper(unit), "UNIT") {
			continue
		}
		names = append(names, unit)
	}
	return names
}

func systemctlShowStates(ctx context.Context, units []string) ([]SystemdService, error) {
	args := []string{"show", "--no-pager",
		"-p", "Id", "-p", "ActiveState", "-p", "SubState", "-p", "UnitFileState", "-p", "LoadState",
	}
	args = append(args, units...)
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	byID := parseSystemctlShowToMap(out)
	var result []SystemdService
	for _, unit := range units {
		m := byID[unit]
		if m == nil {
			m = map[string]string{}
		}
		result = append(result, SystemdService{
			ID:            unit,
			Unit:          unit,
			UnitFileState: strings.TrimSpace(m["UnitFileState"]),
			ActiveState:   strings.TrimSpace(m["ActiveState"]),
			SubState:      strings.TrimSpace(m["SubState"]),
			LoadState:     strings.TrimSpace(m["LoadState"]),
		})
	}
	return result, nil
}

// parseSystemctlShowToMap maps unit Id -> property map from `systemctl show` output (blank-line separated blocks).
func parseSystemctlShowToMap(data []byte) map[string]map[string]string {
	text := strings.TrimSpace(string(data))
	out := make(map[string]map[string]string)
	if text == "" {
		return out
	}
	for _, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		m := parseKeyValueBlock(block)
		id := strings.TrimSpace(m["Id"])
		if id == "" || !strings.HasSuffix(id, ".service") {
			continue
		}
		out[id] = m
	}
	return out
}

func parseKeyValueBlock(block string) map[string]string {
	m := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(block))
	for sc.Scan() {
		line := sc.Text()
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		k, v := strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
		if k != "" {
			m[k] = v
		}
	}
	return m
}
