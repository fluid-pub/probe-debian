# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.2.0] - 2026-06-04

### Added

- Control plane **runtime_config** fetch at startup (`GET /probes/config`) and reload on ping **`configuration_changed`** via **probe-core** `RuntimeSync`.
- Merge of remote `collection`, `files`, `directories`, and `data.entities` intervals via `internal/config/runtime_overlay.go` (`config/runtime_config.example.yml`).

### Changed

- **probe-core** pinned to **`0.2.1`** (CI/release `core_ref` and submodule commit).

## [0.1.0] - 2026-05-26

### Added

- Initial public release: Debian host probe (`debian-probe`), inventory entities (`debian_*`), enrollment via `fluid/probes/core/enroll`, push to control plane.
