.PHONY: all build build-ui test clean sync-skills check-skills bin-homectl bin-mcp-sonos bin-sync-skills install-mcp diagrams

BIN_DIR := ./bin

GIT_TAG_VERSION := $(shell git describe --tags --dirty 2>/dev/null | sed 's/^v//')
VERSION ?= $(if $(GIT_TAG_VERSION),$(GIT_TAG_VERSION),0.4.0)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
           -X github.com/ghchinoy/homectl/pkg/version.Version=$(VERSION) \
           -X github.com/ghchinoy/homectl/pkg/version.GitCommit=$(GIT_COMMIT) \
           -X github.com/ghchinoy/homectl/pkg/version.BuildDate=$(BUILD_DATE)
HOMECTL_BUILD_TAGS := $(shell [ -f ui/dist/index.html ] && echo "-tags embed_ui")

all: build

# Build all binaries into ./bin (gitignored); embeds ui/dist automatically when present
build: bin-homectl bin-mcp-sonos bin-sync-skills

# Compile the Lit + Vite Web UI bundle into ui/dist for embedding or static serving
build-ui:
	npm --prefix ui ci
	npm --prefix ui run build

# Compile Graphviz architecture diagrams (.dot -> .webp)
diagrams:
	@./scripts/build-diagrams.sh

bin-homectl:
	@mkdir -p $(BIN_DIR)
	go build $(HOMECTL_BUILD_TAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/homectl .

bin-mcp-sonos:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/mcp-sonos ./cmd/mcp-sonos

bin-sync-skills:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sync-skills ./cmd/sync-skills

# Synchronize canonical skills/ into self-contained plugins/*/skills/
sync-skills:
	go run ./cmd/sync-skills

# Verify skills synchronization and assert zero missing script references (CI gate)
check-skills:
	go run ./cmd/sync-skills --check

# Run race-enabled test suites across the workspace and modules
test:
	go test -race ./... ./modules/core/... ./modules/sonos/...

clean:
	rm -rf $(BIN_DIR)

# Install MCP servers to ~/.local/bin and register in OpenCode configuration
install-mcp:
	./scripts/install-mcp.sh
