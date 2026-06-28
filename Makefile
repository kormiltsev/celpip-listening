.PHONY: help run build clean \
	build-generator-darwin build-generator-linux \
	build-server-darwin build-server-linux

GEN_APP := generate-kokoro
GEN_SRC := generate-kokoro.go
INPUT := ./dialogue.json

SERVER_APP := static-server
SERVER_SRC := static-server.go

help: ## Show available commands
	@echo "Available targets:"
	@awk 'BEGIN {FS=":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run Kokoro generator
	go run $(GEN_SRC) -in $(INPUT)

build: \
	build-generator-darwin \
	build-generator-linux \
	build-server-darwin \
	build-server-linux ## Build all binaries

build-generator-darwin: ## Build generate-kokoro for macOS ARM64
	GOOS=darwin GOARCH=arm64 go build \
		-ldflags="-s -w" \
		-o $(GEN_APP)-darwin-arm64 \
		$(GEN_SRC)

build-generator-linux: ## Build generate-kokoro for Linux AMD64
	GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-o $(GEN_APP)-linux-amd64 \
		$(GEN_SRC)

build-server-darwin: ## Build static-server for macOS ARM64
	GOOS=darwin GOARCH=arm64 go build \
		-ldflags="-s -w" \
		-o $(SERVER_APP)-darwin-arm64 \
		$(SERVER_SRC)

build-server-linux: ## Build static-server for Linux AMD64
	GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-o $(SERVER_APP)-linux-amd64 \
		$(SERVER_SRC)

clean: ## Remove built binaries
	rm -f \
		$(GEN_APP)-darwin-arm64 \
		$(GEN_APP)-linux-amd64 \
		$(SERVER_APP)-darwin-arm64 \
		$(SERVER_APP)-linux-amd64