BINARY_NAME=debian-probe
BUILD_DIR=build
CONFIG_DIR=config

GO=go
VERSION?=0.1.0
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: all deps build clean test fmt help monorepo-replace

monorepo-replace:
	$(GO) mod edit -replace fluid/probes/core=../core
	$(GO) mod edit -replace fluid/agents/core=../../agents/core
	@$(GO) mod tidy

deps:
	@test -d core || test -d ../core || (echo "Run: git submodule update --init --recursive" && exit 1)
	$(GO) mod download
	@$(GO) mod tidy

build: deps
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

test: deps
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

help:
	@echo "Targets: deps build test fmt monorepo-replace"
