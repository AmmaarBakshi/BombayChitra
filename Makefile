# BombayChitra — common tasks. Windows users: see scripts/*.ps1 or use WSL / Git Bash.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/AmmaarBakshi/BombayChitra/server/internal/httpapi.Version=$(VERSION)
BIN     := bin/bombaychitra

.PHONY: all data web server run dev test lint docker docker-pipeline clean help

all: web server ## Build the web app and the server binary

data: ## Build tiles + database from OpenStreetMap (needs Java 21+)
	./scripts/build-data.sh

web: ## Build the web app and docs into the server's embed directory
	cd web && npm ci && npm run build

server: ## Build the single-binary server (embeds the web app if built)
	cd server && CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/bombaychitra

run: server ## Run the server on :8080 with data/build
	./$(BIN) serve

dev: ## Run the Vite dev server (proxying API/tiles to a server on :8080)
	cd web && npm run dev

test: ## Run all tests and type checks
	cd server && go vet ./... && go test ./...
	cd web && npm run typecheck

docker: ## Build the server image
	docker build -t bombaychitra:$(VERSION) --build-arg VERSION=$(VERSION) .

docker-pipeline: ## Build the data pipeline image
	docker build -t bombaychitra-pipeline:$(VERSION) -f pipeline/Dockerfile .

clean: ## Remove build outputs (keeps downloaded sources)
	rm -rf bin pipeline/build web/dist data/build
	find server/internal/webui/dist -mindepth 1 ! -name .gitkeep -delete

help:
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-16s %s\n", $$1, $$2}'
