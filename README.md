# llm-desk

`llm-desk` is a terminal-based session manager for operators running multiple LLM conversations in parallel across vendors such as Claude, ChatGPT, and Gemini. Rate-limit state is tracked per account, so `llm-desk` tells you when an account is available again; each session keeps its own name, an optional project, a URL, and an ordered queue of prompts to paste into the chat, so you can switch to another account immediately and lose no context.

## Features

- Track rate-limit reset times (`PauseUntil`) per account, independent of how many sessions run under it
- Persistent sessions per account, each with a name, an optional project, a URL, a status, and an ordered queue of prompts
- Grant projects to accounts, with per-project instructions and descriptions; a session or note may reference one granted project
- Many-to-many account ↔ project relationships; project support is per-vendor
- Manage vendors, users, accounts, sessions, projects, and notes from a keyboard-driven terminal interface
- Atomic JSON persistence; no hidden default data path

## Requirements

- Go 1.26 or later
- A terminal emulator with UTF-8 support
- A terminal size of at least 80x24 characters
- No external clipboard tool is required

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

Seven entity types managed by the registry:

- **Vendor**: an LLM service (e.g. `claude`, `chatgpt`). Flags: `HasProjects`, `HasDeepResearch`.
- **User**: identified by email address. Display name is mutable. Managed through its own screen; created explicitly.
- **Account**: a unique (user, vendor) pairing. The owner of sessions and notes, and the holder of pause state (`PauseUntil`) for rate-limit tracking. One account exists per (user, vendor) pair.
- **Session**: a persistent conversation instance under an account; multiple sessions may exist per account at once. Tracks a name, an optional project (must be granted to the account), a URL, a status (active, needs closing, closed), an updated-at timestamp, and an ordered queue of prompts. Session IDs are persisted surrogate keys and are never reused.
- **Prompt**: one queued text item in a session's queue. A prompt stores no account or project; it inherits both from its session.
- **Note**: a stored future work item. It may reference a project and an account, but never a session. Notes are visible to an account according to the project and account assigned to them.
- **Project**: a named context with description and instructions. Granted to accounts many-to-many; only accounts on vendors with `HasProjects` can receive grants. A session or note may reference one project from among those granted to its account.

Removing an account requires ending all its sessions, revoking all its granted projects, and deleting all its notes first. Removing a project requires ending its sessions, deleting its notes, and revoking its grants first. Removing a vendor or user requires removing every account that references it first. No cascading deletes anywhere in the chain.

## Architecture

| Path                    | Responsibility                                                                                             |
| ----------------------- | ---------------------------------------------------------------------------------------------------------- |
| `internal/model/`       | Domain types, `Registry`, validation; no I/O                                                               |
| `internal/apperr/`      | Generic error-category sentinels shared across every layer                                                 |
| `internal/service/`     | Application layer: resolves UI input into domain entities, delegates to `Registry`                         |
| `internal/store/`       | `Store` interface, shared across all `cmd/` frontends                                                      |
| `internal/store/json/`  | JSON file store (imported as `storejson`)                                                                  |
| `cmd/tui/`              | `tview`-based terminal UI                                                                                  |
| `cmd/tui/views/`        | Shared view contract, input, rendering, and modal helpers                                                  |
| `cmd/tui/views/forms/`  | Modal forms and editors; reads and mutates only through `service.Service`, never `model.Registry` directly |
| `cmd/tui/views/panels/` | List panels, account info, and session views                                                               |

Serialisation is handled entirely by the store layer using internal DTO types. Model structs carry no JSON tags.

## Project Structure

```text
llm-desk/
├── cmd/
│   └── tui/
│       └── views/
│           ├── forms/
│           └── panels/
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

Run the full validation suite:

```bash
task check
```

Individual tasks: `task lint`, `task test:full`, `task vuln`, and `task cover`. Tests use table-driven format with `t.Parallel()` at every level.

## Roadmap

- SQLite storage backend

## License

Proprietary. All rights reserved.
