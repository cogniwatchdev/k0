# ─────────────────────────────────────────────
#  K-0 Makefile
# ─────────────────────────────────────────────

BINARY_NAME  := k0
BUILD_DIR    := build
CMD_PATH     := ./cmd/k0
GO           := go
GOFLAGS      :=

.PHONY: all build run test clean install lint tidy

all: build

## build: compile the binary
build:
	@echo "→ Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "✅ Binary: $(BUILD_DIR)/$(BINARY_NAME)"

## run: build and run in dev mode
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

## install: install to /usr/local/bin (requires sudo)
install: build
	@echo "→ Installing to /usr/local/bin/$(BINARY_NAME)..."
	@install -m 755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "✅ Installed"

## test: run all tests
test:
	$(GO) test ./... -v

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: tidy go modules
tidy:
	$(GO) mod tidy

## clean: remove build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@echo "✅ Cleaned"

## help: print this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
