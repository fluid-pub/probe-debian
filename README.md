# fluid-pub/probe-debian

Fluid probe for Debian-like hosts: system metrics, packages, systemd, file checks. Pushes entities to the control plane over HTTP (`/probes`). Authenticates with the public **organization UUID** and probe **connection token** (same identifiers as in the dashboard).

## Repository layout

| Path | Role |
|------|------|
| `core/` | Git submodule → [`fluid-pub/probe-core`](https://github.com/fluid-pub/probe-core) |
| `cmd/` | Entrypoint and `cmd/version.go` (semver for releases) |
| `internal/` | Host collection, config, HTTP shipper |
| `config/probe.example.yml` | Configuration template |
| `config/schema.yml` | Entity schema (shipped in the Docker image) |
| `.github/workflows/` | CI and release via [`fluid-pub/actions`](https://github.com/fluid-pub/actions) |

## Local development

One-time per clone, enable the same **`gofmt`** check as CI:

```bash
./scripts/install-git-hooks.sh
```

```bash
git submodule update --init --recursive
cp config/probe.example.yml config/probe.yml
cp env.secrets.example env.secrets
# Set control plane values in env.secrets (never commit that file).
source env.secrets
go test ./...
go run ./cmd -config config/probe.yml
```

## Features

- CPU, RAM, disk metrics (via gopsutil)
- `os_maintenance`: reboot required (Debian `/var/run/reboot-required`, optional `.pkgs` list, truncated)
- File and directory checks (metadata + SHA-256 where configured)
- APT upgradable packages (`apt list --upgradable`)
- Installed packages inventory (`dpkg-query`, entity type `debian_installed_packages`, stable `id` = `name:architecture`)
- Enabled systemd services (`systemctl list-unit-files` for `enabled` and `enabled-runtime`, then `systemctl show`; entity type `debian_systemd_services`, `id` = unit name)
- HTTP transport under **`/probes`** (not `/agents`): register, ping, ingest
- Local JSONL spool for offline retry
- Optional **enrollment** via `POST /api/v1/enrollment/enroll` (same env pattern as the Linux execution agent)

## HTTP endpoints (control plane)

- `POST /probes/register/:organization_uuid/:token` — record initial liveness in the control plane (idempotent)
- `POST /probes/ping/:organization_uuid/:token` — heartbeat
- `POST /probes/v1/ingest/:organization_uuid/:token` — push snapshot JSON

## Ingest payload

The probe sends JSON (often wrapped in a top-level `state` key) compatible with `ProbeSnapshots`:

```json
{
  "state": {
    "probe": "debian-probe-prod-01",
    "version": "0.2.0",
    "timestamp": "2026-04-07T10:11:12Z",
    "identity": {
      "host_id": "vm-123",
      "hostname": "debian-prod-1"
    },
    "data": {
      "entities": {
        "debian_system_metrics": [{}],
        "debian_file_checks": [],
        "debian_package_updates": [{}],
        "debian_installed_packages": [
          {
            "id": "bash:amd64",
            "name": "bash",
            "version": "5.2.15-2+b2",
            "architecture": "amd64"
          }
        ],
        "debian_systemd_services": [
          {
            "id": "nginx.service",
            "unit": "nginx.service",
            "unit_file_state": "enabled",
            "active_state": "active",
            "sub_state": "running",
            "load_state": "loaded"
          }
        ]
      }
    }
  }
}
```

## Configuration

- Main file: probe identity, collection intervals, file rules (see `config/probe.example.yml`).
- `collection.installed_packages_interval` (default `1h`) and `collection.services_interval` (default `30m`) control how often full package and enabled-service snapshots are pushed.
- Optional durable secrets: `auth.organization_uuid`, `auth.token`, `controlplane.base_url` in `/etc/fluid-probe/credentials.yaml` (0600). See `env.secrets.example` for enrollment bootstrap.

## Snapshots, evolution, and future event streams

The control plane stores each ingest as a snapshot and indexes **entities** with a stable **`fluid`** derived from the payload `id` (or `name`) when present.

- **Package installed or removed** between two snapshots: the entity `fluid` appears or disappears, so snapshot **evolution** views that compare entity sets per type will surface **added** / **removed** rows.
- **Service runtime state change** (same unit, e.g. `active` → `failed`): the `fluid` is unchanged; built-in evolution based only on **added/removed** fluids does **not** flag attribute changes. A future **event** layer (or attribute-level diff) can compare consecutive snapshots on `debian_systemd_services` using the normalized fields `active_state`, `sub_state`, `unit_file_state`, and `load_state`.

The probe keeps these fields stable and explicit so downstream jobs do not need to re-parse `systemctl` output.

## Enrollment (first boot)

1. Create an enrollment token that allows `{ "principal": "probe", "agent_type": "debian" }`.
2. On the host, set `FLUID_ENROLLMENT_TOKEN` and `FLUID_CONTROLPLANE_HTTP_BASE` (e.g. systemd `EnvironmentFile` at `/etc/fluid-probe/enrollment.env`).
3. Run `fluid-probe` with `-credentials /etc/fluid-probe/credentials.yaml` (default). On success, credentials are written and the env file is removed.

Flags: `-config`, `-credentials`, `-enrollment-env` (see `cmd/main.go`).

Report security issues via [`SECURITY.md`](SECURITY.md) (private vulnerability reporting on GitHub).

The enroll API returns **`organization_uuid`** (public); the probe stores that value for all HTTP paths.
