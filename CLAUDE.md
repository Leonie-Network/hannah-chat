# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`hannah-chat` is a terminal chat client for [Hannah](https://github.com/NurPech/Hannah), a self-hosted voice assistant. It connects to a running Hannah Core over gRPC and is the keyboard-only equivalent of Hannah's Telegram bot. Full architecture/protocol docs for the Hannah project live at hannah-docs.leonie.network; this repo is just the client.

## Commands

```sh
go build -o hannah-chat ./cmd/chat   # build
go vet ./...                         # lint (also run in CI)
go test ./...                        # all tests
go test ./cmd/chat -run TestName     # single test
gofmt -l .                           # check formatting (gofmt -w to fix)
```

Requires Go 1.26+ (see `go.mod`). CI (`.github/workflows/test-and-release.yml`) runs `go vet` + `go test` on every push/PR to `main`, and cross-compiles linux/windows amd64 binaries attached to GitHub Releases on `v*` tags.

Run locally against a real Hannah Core: copy `config.example.yaml` to `config.yaml`, set `hannah.address`, then `hannah-chat --config config.yaml`. Every config value can also come from `HANNAH_CHAT_<PATH>` env vars (`.` → `__`), see `internal/config/config.go`.

## Architecture

- **`internal/hannah`** — thin gRPC client wrapping the generated `pb "github.com/NurPech/hannah-proto-go/v4"` stub (`Client` in `client.go`: `SubmitText`, `Login`, `GetDevices`, `ControlDevice`, `LinkAccount`). All outgoing calls get an `x-proto-version` metadata header via interceptors in `version_interceptor.go`, checked server-side against Hannah Core's own `PROTO_VERSION`; `compat_version` (proto-level) runs additively alongside this for narrower, per-message breaking changes.
- **`internal/config`** — YAML config loading with reflection-based env var overrides (no hardcoded allowlist; new config fields automatically get a `HANNAH_CHAT_...` override).
- **`cmd/chat`** — the interactive shell, all `package main`:
  - `main.go` — the read loop. Plain text goes straight to `client.SubmitText`; a leading `/` dispatches a local command; while a `/devices` menu is open, every line is consumed by the menu instead (modal).
  - `session.go` — `session` struct: holds the live client, the shared `bufio.Scanner` (so a command can prompt for more input mid-flow, e.g. `/login`'s username/password), the current identity (`sourceService`/`sourceUserID`, default anonymous `"chat"`), `trustLevel`, and `menu` state. One instance, created in `main()`, threaded through everything by pointer.
  - `commands.go` — local-command registry. Each `/name` is a `localCommand{name, help, trustLevel, run}` registered via `registerCommand` in a file's `init()`; add a new command by creating `cmd_whatever.go` with an `init()` that registers it — nothing else needs to change. `dispatchLocalCommand` enforces `trustLevel` before calling `run`.
  - `cmd_login.go`, `cmd_misc.go`, `cmd_devices.go` — individual command implementations (`/login`, `/logout`, `/exit`, `/devices`).
  - `menu.go` — the `/devices` state machine (rooms → devices → actions → value input/enum select), mirroring the Telegram bot's inline house-control menu (`telegram/hannah_telegram/bot.py` in the main Hannah repo — kept in sync manually, e.g. `deviceMenuTrustLevel`, `categoryIcons`). Room/device identity is tracked as an index into a freshly re-fetched `GetDevices` response on every screen (same trade-off the Telegram bot makes), so the menu tolerates devices changing underneath it but never caches device data across screens.

### Login/identity flow

`/login` calls `Login`, then self-links the roomie's own user ID under provider `"chat"` via `LinkAccount` (if not already linked) so future `SubmitText` calls with `source_service="chat"` resolve back to that user. `session.sourceUserID` and `session.trustLevel` are updated accordingly; `/logout` resets both to the anonymous defaults.

## Conventions

- LF line endings are enforced repo-wide via `.gitattributes`; don't introduce CRLF.
- User-facing strings in this client are English (Telegram bot data like `categoryIcons` keys are German because they come verbatim from ioBroker — that's data, not translated UI text, and stays as-is).
- Comments explaining *why* often reference upstream issues (`gessinger/voice/hannah#NNN`) or the corresponding Python code in the main Hannah repo — check there before assuming behavior is arbitrary.
