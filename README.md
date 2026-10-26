# llm-desk

`llm-desk` is a terminal-based session manager for operators running multiple LLM conversations in parallel across vendors such as Claude, ChatGPT, and Gemini. Rate-limit state is tracked per account, so `llm-desk` tells you when an account is available again; each active session keeps its own topic and a list of pending messages to resume from, so you can switch to another account immediately and lose no context.

## Features

- Track rate-limit reset times (`PauseUntil`) per account, independent of how many sessions run under it
- Multiple ephemeral sessions per account, each with a topic and a list of pending messages
- Grant projects to accounts, with per-project instructions and descriptions; a session may reference one granted project
- Many-to-many account ↔ project relationships; project support is per-vendor
- Manage vendors, users, accounts, sessions, and projects from a keyboard-driven terminal interface
- Atomic JSON persistence; no hidden default data path

## Requirements

- Go 1.26 or later
- A terminal emulator with UTF-8 support

## Installation

```bash
git clone https://github.com/balinomad/llm-desk.git
cd llm-desk
go build -o llm-desk ./cmd/tui
```

## Usage

```bash
./llm-desk --data /path/to/registry.json
./llm-desk -d /path/to/registry.json
```

The `--data` / `-d` flag is required. If the file does not exist, an empty registry is created. Parent directories are created automatically.

## Data Model

Five entity types managed by the registry:

- **Vendor** — an LLM service (e.g. `claude`, `chatgpt`). Flags: `HasProjects`, `HasDeepResearch`.
- **User** — identified by email address. Display name is mutable. Managed through its own screen; created explicitly.
- **Account** — a unique (user, vendor) pairing. The owner of sessions and the holder of pause state (`PauseUntil`) for rate-limit tracking. One account exists per (user, vendor) pair.
- **Session** — an ephemeral conversation instance under an account; multiple sessions may exist per account at once. Tracks a topic, an optional project (must be granted to the account), and a list of pending messages, each optionally flagged to spawn a new session. Session IDs are surrogate keys, not persisted — renumbered sequentially on each load.
- **Project** — a named context with description and instructions. Granted to accounts many-to-many; only accounts on vendors with `HasProjects` can receive grants. A session may reference one project from among those granted to its account.

Removing an account requires ending all its sessions and revoking all its granted projects first. Removing a vendor or user requires removing every account that references it first. No cascading deletes anywhere in the chain.

## Architecture

| Path                   | Responsibility                                                                                               |
| ---------------------- | ------------------------------------------------------------------------------------------------------------ |
| `internal/model/`      | Domain types, `Registry`, validation — no I/O                                                                |
| `internal/apperr/`     | Generic error-category sentinels shared across every layer                                                   |
| `internal/service/`    | Application layer: resolves UI input into domain entities, delegates to `Registry`                           |
| `internal/store/`      | `Store` interface, shared across all `cmd/` frontends                                                        |
| `internal/store/json/` | JSON file store (imported as `storejson`)                                                                    |
| `cmd/tui/`             | `tview`-based terminal UI                                                                                    |
| `cmd/tui/views/`       | Panels, forms, and modals; reads and mutates only through `service.Service`, never `model.Registry` directly |

Serialisation is handled entirely by the store layer using internal DTO types. Model structs carry no JSON tags.

## Project Structure

```text
llm-desk/
├── cmd/
│   └── tui/
│       └── views/
├── internal/
│   ├── apperr/
│   ├── model/
│   ├── service/
│   └── store/
│       └── json/
└── go.mod
```

## Storage

State is persisted to a single JSON file. Writes are atomic: the file is written to a temporary sibling first, then renamed into place. A SQLite backend is planned but deferred.

## Development

```bash
go test ./...
```

Tests use table-driven format with `t.Parallel()` at every level.

## Roadmap

- SQLite storage backend

## License

Proprietary. All rights reserved.
