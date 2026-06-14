# llm-desk

`llm-desk` is a terminal-based session manager for operators running multiple LLM conversations in parallel across providers such as Claude, ChatGPT, and Gemini. When a session hits a rate limit, `llm-desk` tells you when you can resume, where you left off, and what to ask next — so you can switch to another project immediately and lose no context.

## Features

- Track rate-limit reset times and next-prompt context per session
- Log topic, mid-answer state, and next prompt for each session's last question
- Attach sessions to projects with per-project instructions and descriptions
- Many-to-many session ↔ project relationships; project support is per-provider
- Manage providers, users, sessions, and projects from a keyboard-driven terminal interface
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

Four entity types managed by the registry:

- **Provider** — an LLM service (e.g. `claude`, `chatgpt`). Flags: `HasProjects`, `HasDeepResearch`.
- **User** — identified by email address. Display name is mutable.
- **Session** — a (user, provider) pair. Tracks when the session is available again (`NextPromptAt`) and the last question's topic, mid-answer state, next prompt, and optional project context.
- **Project** — a named context with description and instructions. Attached to sessions many-to-many; only sessions on providers with `HasProjects` can receive attachments.

Removing a provider or user requires removing all sessions that reference them first — no cascades.

## Architecture

| Path                   | Responsibility                                        |
| ---------------------- | ----------------------------------------------------- |
| `internal/model/`      | Domain types, `Registry`, validation — no I/O         |
| `internal/store/`      | `Store` interface, shared across all `cmd/` frontends |
| `internal/store/json/` | JSON file store (imported as `storejson`)             |
| `cmd/tui/`             | `tview`-based terminal UI                             |
| `cmd/tui/views/`       | Panels, forms, and modals                             |

Serialisation is handled entirely by the store layer using internal DTO types. Model structs carry no JSON tags.

## Project Structure

```text
llm-desk/
├── cmd/
│   └── tui/
│       └── views/
├── internal/
│   ├── model/
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
