SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

BACKEND := backend
CLIENT := client

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  %-20s %s\n", $$1, $$2}'

# ---- backend ---------------------------------------------------------------
.PHONY: backend backend-release backend-windows backend-test backend-race backend-e2e coverage bench
backend: ## Build backend debug binary -> backend/bin/databackup
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath -o bin/databackup ./cmd/databackup

backend-release: ## Build backend release binary (-s -w)
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/databackup ./cmd/databackup

backend-windows: ## Cross-compile Windows backend -> backend/bin/databackup.exe (C4b)
	cd $(BACKEND) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o bin/databackup.exe ./cmd/databackup

backend-test: ## Run backend unit/integration tests
	cd $(BACKEND) && go test ./...

backend-race: ## Run backend tests with the race detector
	cd $(BACKEND) && go test -race ./...

backend-e2e: ## Run backend e2e tests (build tag e2e)
	cd $(BACKEND) && go test -tags e2e ./e2e/...

coverage: ## Backend coverage report -> build/coverage/backend.html
	mkdir -p build/coverage
	cd $(BACKEND) && go test -coverprofile=../build/coverage/backend.out ./...
	cd $(BACKEND) && go tool cover -html=../build/coverage/backend.out -o ../build/coverage/backend.html

bench: ## Run backend benchmarks
	cd $(BACKEND) && go test -bench=. -run='^$$' ./...

# ---- client ----------------------------------------------------------------
.PHONY: client-install client client-test client-lint e2e
client-install: ## npm ci (respects package-lock.json)
	cd $(CLIENT) && npm ci

client: ## Build the Electron client -> client/out/
	cd $(CLIENT) && npm run build

client-test: ## Client unit tests (vitest)
	cd $(CLIENT) && npm test

client-lint: ## Client eslint + prettier check + typecheck
	cd $(CLIENT) && npm run lint && npm run format:check && npm run typecheck

e2e: backend client ## Client e2e: launch built app against real backend (own Xvfb if no DISPLAY)
	cd $(CLIENT) && DATABACKUP_BIN="$(CURDIR)/$(BACKEND)/bin/databackup" \
		bash ../scripts/e2e-display.sh npm run test:e2e

# ---- format / lint ---------------------------------------------------------
.PHONY: format format-check lint
format: ## Format backend (gofmt -w) + client (prettier --write)
	bash scripts/format.sh
	cd $(CLIENT) && npm run format

format-check: ## gofmt --check + prettier --check
	bash scripts/format.sh --check
	cd $(CLIENT) && npm run format:check

lint: ## go vet + staticcheck + check-imports + client eslint/typecheck
	bash scripts/lint.sh
	cd $(CLIENT) && npm run lint && npm run typecheck

# ---- gates -----------------------------------------------------------------
.PHONY: check check-all
check: format-check lint backend backend-test client client-test ## Fast gate (run before commit)

check-all: check backend-race backend-e2e e2e ## Full gate: check + race + backend e2e + client e2e

# ---- docs / package --------------------------------------------------------
.PHONY: docs package clean
docs: ## Build docs PDFs -> docs/pdf/ (needs docs toolchain or docker-docs)
	bash scripts/docs/build-docs.sh

package: backend-release client-install ## Release artifacts -> dist/
	cd $(CLIENT) && npm run package
	mkdir -p dist
	cp $(BACKEND)/bin/databackup dist/
	cp $(CLIENT)/release/* dist/ 2>/dev/null || true

clean: ## Remove build outputs (keeps docs/pdf and node_modules)
	rm -rf build dist $(BACKEND)/bin $(CLIENT)/out $(CLIENT)/release

# ---- docker wrappers (implemented in scripts/docker.sh; works without make
# ---- on the host: `bash scripts/docker.sh check` etc.) -----------------------
.PHONY: docker-dev-image docker-docs-image docker-check docker-check-all docker-shell docker-docs
docker-dev-image: ## Build the dev toolchain image
	bash scripts/docker.sh images dev

docker-docs-image: ## Build the docs toolchain image
	bash scripts/docker.sh images docs

docker-check: ## Run `make check` inside the dev image (Windows host path)
	bash scripts/docker.sh check

docker-check-all: ## Run `make check-all` inside the dev image
	bash scripts/docker.sh check-all

docker-shell: ## Open a shell in the dev image
	bash scripts/docker.sh shell

docker-docs: ## Build docs PDFs inside the docs image -> docs/pdf/
	bash scripts/docker.sh docs
