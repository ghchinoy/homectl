.PHONY: all build test clean sync-skills check-skills bin-homectl bin-mcp-sonos bin-sync-skills install-mcp

BIN_DIR := ./bin

all: build

# Build all binaries into ./bin (gitignored)
build: bin-homectl bin-mcp-sonos bin-sync-skills

bin-homectl:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/homectl .

bin-mcp-sonos:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/mcp-sonos ./cmd/mcp-sonos

bin-sync-skills:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/sync-skills ./cmd/sync-skills

# Synchronize canonical skills/ into self-contained plugins/*/skills/
sync-skills:
	go run ./cmd/sync-skills

# Verify skills synchronization and assert zero missing script references (CI gate)
check-skills:
	go run ./cmd/sync-skills --check

# Run test suites across the workspace
test:
	go test -v ./...

clean:
	rm -rf $(BIN_DIR)

# Install MCP servers to ~/.local/bin and register in OpenCode configuration
install-mcp:
	./scripts/install-mcp.sh
