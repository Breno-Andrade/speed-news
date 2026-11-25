# Speed News

Tray widget (not a desktop window) that aggregates Formula 1 news and shows the current race weekend schedule. No login.

## Layout

- `apps/desktop/` — Tauri v2 shell + React/TS UI. Rust in `src-tauri/` is only glue: tray, popover window, sidecar lifecycle.
- `apps/api/` — Go backend, runs as a Tauri sidecar. All business logic lives here. Outbound clients live only in `internal/providers/*`.
- Flows (startup, tray updates, popover lifecycle, updates) are documented in `docs/fluxos.md`: keep it current when they change.
- The popover webview is disposable: created on first open, destroyed 60 s after hiding (`popover.rs`). Nothing that must work
  while it is closed may depend on it. The tray is fed by the sidecar's stdout (`SPEED_NEWS_TRAY {json}`, Go `internal/shellbridge`
  ↔ Rust `sidecar.rs`/`tray.rs`); new fields need defaults on both sides. Never create windows inside event handlers or sync
  commands (Windows deadlock): use a separate thread like `popover::show_popover`. Popover placement lives in `placement.rs`
  (pure, tested per monitor layout); never size it from `outer_size()` (0×0 on a GTK window that was never mapped).
- `tools/icon-generator/` — single source of every icon: app icon, tray symbol (light/dark, each with an unread red dot) and the
  GNOME panel SVG. Never edit generated icons by hand: change the generator and run `make icons` (a golden test compares them).
  Linux/Windows show the red dot (count in the popover header); macOS shows the count as menu bar text (`set_title`).
- First open must go skeleton → content in place: `index.html` inlines `src/StartupSkeleton.tsx` (run
  `npm run generate:startup-skeleton` after changing any skeleton; a test fails otherwise) and links `styles.css` directly.
  Keep skeleton geometry equal to the real card/list (the E2E "first paint" test checks the filters never move).
- Tray texts are duplicated in `src-tauri/src/tray_texts.rs` and `messages.ts` (a Vitest parity test fails if they drift).
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
- Coverage floors (Go 80%, frontend in `vite.config.ts`) only go up.
- Long-running behaviour has tests: `TestRepeatedRefreshCyclesReachASteadyState` (goroutines/heap/DB file stay flat), SQLite shrink
  and legacy auto_vacuum migration. Keep them passing when touching refresh, storage or HTTP clients.
- Memory: `make measure-memory` (Linux) / `scripts/measure-memory.ps1` (Windows) per process group.

## Releases / updater

- Version lives only in `apps/desktop/package.json`; tag `v*` → `release.yml` (draft release + signed bundles + `latest.json`).
- Updater logic: `src-tauri/src/updates.rs` (silent on AppImage/Windows/macOS while the popover is hidden; ask first on .deb/.rpm; never in dev builds).
- Updater signing key: never commit it. It lives in the maintainer's `~/.tauri/` and in the repo secrets; the public key is in `tauri.conf.json`.
- Keep the D-Bus interface `dev.speednews.Desktop1` backward compatible: the GNOME extension updates separately from the app.
- Data must survive updates: schema changes go through `addedColumns`/defaults in `storage/sqlite`, settings fields need defaults.
- Updater/UI events can fire while the webview does not exist: the UI must also ask for the state on mount (see `pending_update`).

## Toolchain

Pinned versions live in `.devcontainer/Dockerfile`, `.mise.toml`, `rust-toolchain.toml` and `.github/workflows/*.yml`. Bump them together.

## Commands

- `make test` / `make lint`
- `make backend-dev` + `make web-dev` — UI in the browser, no Rust required
- `make dev` — full desktop app
- `make docker-check` / `make docker-bundle-linux` — same checks / Linux bundles inside the pinned image
