.PHONY: all build test clean sync-skills check-skills

BIN_DIR := ./bin

all: build

# Build all binaries into ./bin (gitignored)
build: $(BIN_DIR)/homectl $(BIN_DIR)/mcp-sonos $(BIN_DIR)/sync-skills

$(BIN_DIR)/homectl:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/homectl .

$(BIN_DIR)/mcp-sonos:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/mcp-sonos ./cmd/mcp-sonos

$(BIN_DIR)/sync-skills:
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
