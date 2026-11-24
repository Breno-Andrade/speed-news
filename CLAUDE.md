# Speed News

Tray widget (not a desktop window) that aggregates Formula 1 news and shows the current race weekend schedule. No login.

## Layout

- `apps/desktop/` — Tauri v2 shell + React/TS UI. Rust in `src-tauri/` is only glue: tray, popover window, sidecar lifecycle.
- `apps/api/` — Go backend, runs as a Tauri sidecar. All business logic lives here.
- `tools/icon-generator/` — Go program that draws the app icon and the two tray states (default / unread dot).
  Linux/Windows show the red dot (count in the popover header); macOS shows the count as menu bar text (`set_title`).
  Linux tray runs on the `ksni` backend (forced in `src-tauri/Cargo.toml`) so left click opens the popover; don't remove it.
- `apps/desktop/gnome-shell-extension/` — GNOME panel button (single click opens the news, shows the count). Talks to the app over D-Bus
  (`dev.speednews.Desktop`, implemented in `src-tauri/src/panel_bridge.rs`). Keep the interface XML in `extension.js` in sync with that file.
  Install with `make install-gnome-extension`.

## Rules

- **Never use non-semantic identifiers** (`a`, `x`, `z`, `i`, `e`, `t`, `w`, `r`…), in any language. Loop vars, callbacks, receivers and test params included (`test *testing.T`, `responseWriter`, `request`). Allowed short names: `err`, `ctx`, `id`. Enforced by ESLint `id-length` and golangci-lint `varnamelen`.
- Times are UTC everywhere in the backend and API; convert only for display, in `apps/desktop/src/shared/time/timeZone.ts`.
- News: show only what the feed exposes (title, summary, image, link). Never scrape full articles.
- New news source = new entry in `apps/api/internal/sources/default_sources.yaml`, no code.
- Scope is F1 only for now, but keep `SeriesSlug` on entities so WEC can be added later.

## Tests

- `make check` = what CI runs (lint, coverage floors, E2E). `make test` for the fast loop.
- Tests never hit real sources: use `apps/api/internal/fixtures` (fake RSS/Jolpica/f1-circuits/Open-Meteo, dates relative to now, `/control/*` endpoints). New external source ⇒ add it to the fixtures too.
- Component tests: `// @vitest-environment jsdom`, render with `src/test/renderWithProviders.tsx` and the fake client in `src/test/fakeApiClient.ts`.
- E2E (`apps/desktop/e2e`) shares one backend: each test restores state in `beforeEach`; never depend on test order.
- Coverage floors (Go 78%, frontend in `vite.config.ts`) only go up.

## Toolchain

Pinned versions live in `.devcontainer/Dockerfile`, `.mise.toml`, `rust-toolchain.toml` and `.github/workflows/*.yml`. Bump them together.

## Commands

- `make test` / `make lint`
- `make backend-dev` + `make web-dev` — UI in the browser, no Rust required
- `make dev` — full desktop app
- `make docker-check` / `make docker-bundle-linux` — same checks / Linux bundles inside the pinned image
