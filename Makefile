# general variables
OUT_DIR := ./out
BIN_DIR := ./bin
COVER_FILE := $(OUT_DIR)/coverage.out

# recipes list
.PHONY: fmt lint run clean

# source files for tracking changes
SRC := $(shell find . -type f -name '*.go')

# public recipe for formatting
fmt: $(OUT_DIR)/fmt.cache

# public recipe for linting
lint: $(OUT_DIR)/lint.cache

# public recipe for building
build: $(OUT_DIR)/build.cache

# running
run: build
	@echo ">> Running CLI..."
	@$(BIN_DIR)/cli

# cleaning up garbage
clean:
	@echo ">> Cleaning up..."
	@rm -rf $(BIN_DIR) $(OUT_DIR)
	@echo ">> Cleaned."

# creating output directory if it's missing
$(OUT_DIR):
	@mkdir -p $(OUT_DIR)

# formatting source code
$(OUT_DIR)/fmt.cache: $(SRC) | $(OUT_DIR)
	@echo ">> Formatting..."
	@gofmt -s -w .
	@echo ">> Formatted."
	@touch $@

# linting source code
$(OUT_DIR)/lint.cache: $(SRC) | $(OUT_DIR)
	@echo ">> Linting..."
	@golangci-lint run ./... --timeout=5m > $(OUT_DIR)/lint.log || { \
		cat $(OUT_DIR)/lint.log; \
		exit 1; \
	};
	@echo ">> Linted."
	@touch $@

# building
$(OUT_DIR)/build.cache: $(SRC) | $(OUT_DIR)
	@echo ">> Building CLI..."
	@go build -o $(BIN_DIR)/cli cmd/main.go
	@echo ">> Built in $(BIN_DIR)/cli"
	@touch $@
