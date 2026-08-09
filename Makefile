.DEFAULT_GOAL := help

BINARY      := bin/flagit
WEB_DIST    := web/dist
EMBED_DIST  := internal/overlay/dist
DB_PATH     ?= ./data/flagit.db
DOCKER_TAG  ?= flagit
COVERAGE_MIN ?= 90

# Revision stamped into the binary and shown in the admin dashboard. Resolved
# from the checkout, so a local build names itself honestly; empty outside a
# git checkout, which the dashboard reports as "unknown".
GIT_COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null)
LDFLAGS     := -s -w -X flagit/internal/version.Commit=$(GIT_COMMIT)

.PHONY: help dev dev-web test test-go test-web coverage build web clean docker docker-run fmt vet lint

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[1m%-12s\033[0m %s\n", $$1, $$2}'

dev: ## Run the Go server in dev mode (proxies the frontend to Vite)
	go run ./cmd/flagit --dev --db-path $(DB_PATH)

dev-web: ## Run the Vite dev server (pair with `make dev` in another shell)
	cd web && npm run dev

web: ## Build the Svelte frontend and stage it for embedding
	cd web && npm run build
	rm -rf $(EMBED_DIST)
	mkdir -p $(EMBED_DIST)
	cp -R $(WEB_DIST)/. $(EMBED_DIST)/

build: web ## Build the production binary with the frontend embedded
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/flagit

test: test-go test-web ## Run every test

test-go: ## Run the Go tests
	go test ./... -race

test-web: ## Run the Svelte tests
	cd web && npm run test

coverage: ## Report test coverage for both halves, failing below the target
	go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		awk -v got="$$total" -v want=$(COVERAGE_MIN) 'BEGIN { \
			if (got + 0 < want + 0) { \
				printf "FAIL: Go coverage %.1f%% is below the %s%% target\n", got, want; exit 1 \
			} \
			printf "Go coverage %.1f%% meets the %s%% target\n", got, want \
		}'
	cd web && npm run coverage

fmt: ## Format the Go sources
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Format and vet

docker: ## Build the Docker image
	docker build --build-arg GIT_COMMIT=$(GIT_COMMIT) -t $(DOCKER_TAG) .

docker-run: docker ## Build and run the image locally
	docker run --rm -p 8080:8080 -p 3000:3000 -v flagit_data:/data $(DOCKER_TAG)

clean: ## Remove build output and coverage reports
	rm -rf bin $(WEB_DIST) coverage.out web/coverage
	rm -rf $(EMBED_DIST)
	mkdir -p $(EMBED_DIST)
	printf 'Build output from web/ is copied here by `make web`. The directory must exist\nfor the //go:embed directive in overlay.go to compile.\n' > $(EMBED_DIST)/.gitkeep
