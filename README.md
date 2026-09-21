# hannah-chat

Lightweight terminal chat client for [Hannah](https://github.com/NurPech/Hannah), a self-hosted, German-language voice assistant. Connects straight to Hannah Core over gRPC — a fast, keyboard-only alternative to the Telegram bot for talking to Hannah or controlling devices.

## Features

- Plain text chat with Hannah, through the same NLU/intent pipeline as voice and Telegram
- `/login` / `/logout` — authenticate as a Hannah user; the session carries your trust level
- `/devices` — numbered, nested device-control menu (rooms → devices → actions), the keyboard equivalent of Telegram's inline house-control menu
- Single static binary, no runtime dependencies beyond a reachable Hannah Core instance

## Installation

### Download

Pre-built binaries for Linux and Windows (amd64) are attached to each [release](https://github.com/Leonie-Network/hannah-chat/releases).

### Build from source

```sh
go build -o hannah-chat ./cmd/chat
```

Requires Go 1.26 or newer (see `go.mod`).

## Configuration

Copy `config.example.yaml` to `config.yaml` and point it at Hannah Core's gRPC address:

```yaml
hannah:
  address: 192.168.1.1:50051
```

Every setting can also be set via environment variable as `HANNAH_CHAT_<PATH>` (`.` in the path becomes `__`), e.g. `HANNAH_CHAT_HANNAH__ADDRESS=192.168.1.1:50051` — useful for container deployments without a config file.

## Usage

```sh
hannah-chat --config config.yaml
```

Then just type. Anything not starting with `/` is sent to Hannah as a normal command or question. Available `/` commands:

| Command | Description |
|---|---|
| `/login` | Log in as a Hannah user (prompts for username/password) |
| `/logout` | Forget the current identity, back to anonymous |
| `/devices` | Browse and control devices by room (requires trust level 7); navigate the numbered menu by typing the number shown, `0` to go back or close |
| `/exit` | Quit |

## Part of the Hannah project

hannah-chat talks to [Hannah Core](https://github.com/NurPech/Hannah), the public mirror of Hannah's repo. Full architecture, protocol and setup docs live at [hannah-docs.leonie.network](https://hannah-docs.leonie.network).
