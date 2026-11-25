.PHONY: setup setup-linux dev backend-dev web-dev test test-e2e coverage check lint icons measure-memory build docker-check docker-bundle-linux install-gnome-extension

# Toolchains installed by scripts/setup-linux.sh work even if the shell was never reloaded
# (rustup and mise only update PATH for new shells). mise shims resolve the versions in .mise.toml.
export PATH := $(HOME)/.cargo/bin:$(HOME)/.local/share/mise/shims:$(PATH)

DEV_API_PORT  := 4317
DEV_API_TOKEN := dev-token

# Container-only volumes keep node_modules, Go and Cargo caches out of the host checkout.
# The mount points are created first, by the host user: otherwise Docker creates them as root
# and later native builds (cargo, npm) fail with "Permission denied".
DOCKER_RUN := mkdir -p node_modules apps/desktop/src-tauri/target && docker run --rm -it -v "$(CURDIR)":/workspace \
	-v speed-news-go-cache:/go/pkg \
	-v speed-news-container-node-modules:/workspace/node_modules \
	-v speed-news-cargo-target:/workspace/apps/desktop/src-tauri/target \
	-v speed-news-cargo-registry:/home/developer/.cargo/registry

setup: ## Install JS dependencies (Go modules download on first build)
	npm install

setup-linux: ## Native setup on Debian/Ubuntu: Tauri libs, Rust, Go/Node via mise
	./scripts/setup-linux.sh

GNOME_EXTENSION_UUID := speed-news@speednews.dev
GNOME_EXTENSION_DIR  := $(HOME)/.local/share/gnome-shell/extensions/$(GNOME_EXTENSION_UUID)

install-gnome-extension: ## GNOME only: panel button where a single click opens the news
	rm -rf "$(GNOME_EXTENSION_DIR)"
	mkdir -p "$(GNOME_EXTENSION_DIR)"
	cp -r apps/desktop/gnome-shell-extension/. "$(GNOME_EXTENSION_DIR)/"
	@# GNOME Shell keeps extension code (ES modules) and icons in memory: disable/enable does not
	@# reload them. A new install AND any change to an installed extension need a shell reload.
	@echo "Installed. Reload GNOME Shell to load it (needed after every change):"
	@echo "  X11:     Alt+F2, type r, Enter"
	@echo "  Wayland: log out and back in"
	@echo "then, on the first install only: gnome-extensions enable $(GNOME_EXTENSION_UUID)"

dev: ## Full desktop app (Tauri + sidecar). Requires Rust and the Tauri system libraries.
	npm run dev

backend-dev: ## Backend only, fixed port and token, for `make web-dev`
	SPEED_NEWS_API_TOKEN=$(DEV_API_TOKEN) go run ./apps/api/cmd/news-api \
		--port $(DEV_API_PORT) --data-dir .dev-data --allowed-origins http://localhost:1420 --verbose

web-dev: ## UI in the browser against `make backend-dev` (no Rust needed)
	npm run dev:web

test: ## Fast tests: Go (unit + integration), frontend (unit + component), Rust (if installed)
	go test ./apps/api/... ./tools/icon-generator/...
	npm test
	@if command -v cargo >/dev/null; then cd apps/desktop/src-tauri && cargo test --quiet; \
	else echo "cargo not installed, skipping Rust tests"; fi

test-e2e: ## End-to-end in a real browser: fixture server + Go backend + UI (Playwright)
	npm run test:e2e --workspace apps/desktop

coverage: ## Tests with coverage floors, exactly as CI runs them
	./scripts/check-go-coverage.sh
	npm run test:coverage --workspace apps/desktop

check: lint coverage test-e2e ## Everything CI checks, locally

lint: ## Formatting, vet, golangci-lint (if installed), ESLint and TypeScript
	@test -z "$$(gofmt -l apps/api tools)" || (gofmt -l apps/api tools && echo "run gofmt -w" && exit 1)
	go vet ./apps/api/... ./tools/icon-generator/...
	@# "a && b || c" would print "not installed" and succeed when b FAILS: lint errors were swallowed.
	@if command -v golangci-lint >/dev/null; then golangci-lint run ./apps/api/... ./tools/icon-generator/...; \
	else echo "golangci-lint not installed, skipping (run: mise install)"; fi
	npm run lint
	npm run typecheck

measure-memory: ## Memory of the running app per process group (shell, webview, sidecar)
	./scripts/measure-memory.sh

icons: ## Regenerate the app icon, tray icons and GNOME panel icon (one source: tools/icon-generator)
	npm run icons --workspace apps/desktop

build: ## Installer for the current platform
	npm run build

docker-check: ## Lint + tests inside the pinned dev image (nothing installed on the host)
	docker build -f .devcontainer/Dockerfile --target dev -t speed-news-dev .
	$(DOCKER_RUN) speed-news-dev bash -c "sudo chown developer /workspace/node_modules && npm ci && make lint test"

docker-bundle-linux: ## .deb + AppImage built inside the pinned linux-build image
	docker build -f .devcontainer/Dockerfile --target linux-build -t speed-news-linux-build .
	$(DOCKER_RUN) -e APPIMAGE_EXTRACT_AND_RUN=1 speed-news-linux-build bash -c \
		"sudo chown developer /workspace/node_modules /workspace/apps/desktop/src-tauri/target /home/developer/.cargo/registry \
		&& npm ci \
		&& npm run tauri:build --workspace apps/desktop -- --bundles deb,appimage \
		&& mkdir -p dist-linux && cp -r apps/desktop/src-tauri/target/release/bundle/. dist-linux/"
