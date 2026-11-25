# Speed News 🏁

Tray widget that gathers Formula 1 news and shows the race weekend schedule in your time zone.

- **Tray icon shows when there is something new** (red dot on Linux/Windows, count as text on macOS); left click opens the news, right click shows Open/Quit.
- **Race weekend card**: Grand Prix, circuit drawing and every session grouped by day, in the computer's time zone (or one you pick).
- **News from 6 sources** (EN + PT) via RSS: headline, summary and a link to read the full article on the source site.
- No login; everything is stored locally (SQLite).

## Architecture

```
Tauri (Rust glue: tray, popover, sidecar)
 ├─ WebView: React + TS ──── HTTP + SSE (127.0.0.1:<random port>, random token)
 └─ sidecar: Go news-api ─── RSS feeds · Jolpica F1 calendar · f1-circuits layouts → SQLite
```

| Data | Source | Refresh |
|---|---|---|
| News | RSS feeds in `apps/api/internal/sources/default_sources.yaml` | 30 min · 5 min on race weeks |
| Calendar and sessions | [Jolpica F1 API](https://github.com/jolpica/jolpica-f1) | every 12 h |
| Circuit drawing and facts (length, first GP) | [bacinger/f1-circuits](https://github.com/bacinger/f1-circuits) (MIT), GeoJSON → SVG | cached once per circuit |
| Last GP (podium, pole, fastest lap) and history at the circuit | [Jolpica F1 API](https://github.com/jolpica/jolpica-f1) | cached 3 h / 24 h |
| Weather per session (race week only) | [Open-Meteo](https://open-meteo.com) (CC BY 4.0) | cached 1 h |

External answers are cached in SQLite (`api_cache`): when a source is down, the last answer is shown instead of an error.

## Development environment

Versions are pinned in one place per tool and mirrored everywhere (Dev Container, mise, CI):
Go 1.27.4 · Node 22.23.5 · Rust 1.100.0 (`rust-toolchain.toml`) · golangci-lint 2.15.0.

| What | How | Needs on the host |
|---|---|---|
| Daily work (Go + React, UI in the browser) | **Dev Container**: VS Code → "Reopen in Container", then `make backend-dev` + `make web-dev` | Docker + VS Code |
| Lint + tests without installing anything | `make docker-check` | Docker |
| Linux installers (.deb, AppImage) | `make docker-bundle-linux` → `dist-linux/` | Docker |
| Run the real tray app | `make setup-linux` once, then `make dev` | Debian/Ubuntu |
| Windows / macOS installers | push a `v*` tag → GitHub Actions draft release | — |

The tray app itself is not run inside Docker: it needs the host's display server and panel (D-Bus), which makes containers fragile for GUI work.

On GNOME the tray needs the *AppIndicator* extension (preinstalled on Ubuntu).

## Tests

| Layer | What it covers | Run |
|---|---|---|
| Go unit + integration | services, HTTP clients, storage/migrations, and the whole backend wired by `internal/app` against a fixture server | `go test ./apps/api/...` |
| Frontend unit + component | formatting, time zones, and screens (Vitest + Testing Library, fake API client) | `npm test` |
| Rust | popover positioning and the click-after-focus-loss guard | `cargo test` in `apps/desktop/src-tauri` |
| End-to-end | real UI + real Go backend + fixture server, in Chromium (and WebKit in CI) | `make test-e2e` |

External sources are never called by tests: `apps/api/internal/fixtures` fakes RSS, Jolpica, f1-circuits and Open-Meteo, with dates relative to today (the suite never expires) and control endpoints to publish a story or simulate an outage. Coverage floors: Go 78% (`scripts/check-go-coverage.sh`), frontend in `vite.config.ts`.

`make check` runs exactly what CI runs.

## Commands

```bash
make backend-dev   # terminal 1: backend on :4317
make web-dev       # terminal 2: UI on http://localhost:1420
make dev           # full desktop app (native)
make test
make lint
```

## Platform notes

- **Linux / GNOME (recommended)**: run `make install-gnome-extension` once. It adds a Speed News button to the top bar: single click opens the news right below it, right click shows Open/Quit, and the unread count is shown next to the icon. While the button is active the app hides its tray icon. Why: Ubuntu's AppIndicator extension opens the tray menu on every single click (only a double click reaches the app), and it has no setting to change that.
- **Linux without the extension** (or other desktops): tray icon via the `ksni` backend (StatusNotifierItem). On GNOME/Ubuntu single click opens the menu and double click opens the news; KDE Plasma opens the news on single click. Unread items show as a red dot; the count is in the popover header.
- **macOS**: clicking the icon opens the popover next to it; the count is the menu bar title.
- **Windows**: clicking the icon opens the popover; red dot on the icon, count in the tooltip and popover header.
- Everywhere: the popover has a power button to quit the app.

## Releases and automatic updates

1. Bump `version` in `apps/desktop/package.json` (the only place: `tauri.conf.json` reads it).
2. Commit, then `git tag v0.2.0 && git push --tags`.
3. The Release workflow builds Linux, Windows and macOS, signs every bundle and creates a **draft** release with `latest.json`.
4. Publish the draft. Installed apps find it through `releases/latest/download/latest.json`.

Installed apps check one minute after start and every 6 hours:

| Installed as | Update |
|---|---|
| AppImage (recommended on Linux), Windows, macOS | downloaded in the background, installed while the popover is closed, app restarts; "Updated to vX" shows next time |
| `.deb` / `.rpm` | downloaded, then offered in the app ("Update now"): installing asks for the administrator password |

Updates are signed: the app only installs bundles signed with the project's updater key (public key in `tauri.conf.json`, private key in the repository secrets `TAURI_SIGNING_PRIVATE_KEY` / `TAURI_SIGNING_PRIVATE_KEY_PASSWORD`). **If the private key is lost, installed apps can never update again** — keep a backup.

## License and trademarks

Copyright © 2026 Breno Andrade.

Speed News is free software under the [GNU GPL v3.0 or later](LICENSE): you may use, study, share and modify it, and any distributed version (modified or not) must stay under the GPL, keep this copyright notice and make its source code available.

Speed News is an independent project, not affiliated with, endorsed by or connected to Formula 1, the FIA or any team. "Formula 1", "F1" and related marks are trademarks of Formula One Licensing B.V. News belong to their publishers; the app shows only the headline, summary and image each feed publishes and links to the original article.
