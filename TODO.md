# Todo

## Errors

## Rename

- [ ] Rename the project from `llm-desk` to `prompt-backlog` (before making the repo public; apply one sub-item at a time; run after the README rewording item, or rely on the final grep to catch leftovers in the text that item adds)
  - [ ] Decide the binary name (default `prompt-backlog`, same as the repo); the sub-items below assume the default
  - [ ] Decide whether `go install` must produce `prompt-backlog`: it names the binary after the package directory, so `cmd/tui` installs as `tui`; default is to keep `cmd/tui` (the layout is documented as shared across frontends) and document only `go build -o prompt-backlog ./cmd/tui`
  - [ ] `go.mod`: change the module path with the command below
  - [ ] Rewrite every import of the old module path in the Go files (the sed form below is GNU sed; on macOS use `sed -i ''`), then run `go build ./...`

    ```bash
    go mod edit -module github.com/balinomad/prompt-backlog
    grep -rl 'github.com/balinomad/llm-desk' --include='*.go' . | xargs sed -i 's#github.com/balinomad/llm-desk#github.com/balinomad/prompt-backlog#g'
    ```

  - [ ] `cmd/tui/app.go`: change `const AppName = "llm-desk"` to `"prompt-backlog"` (it feeds the quit dialog, the flag set name, and the error prefix)
  - [ ] `cmd/tui/views/helpers_test.go`: change the `AppName()` return value in `mockApp` to `"prompt-backlog"`
  - [ ] `cmd/tui/main_internal_test.go`: change the expected stderr substring `Usage of llm-desk` to `Usage of prompt-backlog`
  - [ ] `.golangci.yml`: update the header comment, the `depguard` allow entry `github.com/balinomad/llm-desk`, and the `wrapcheck` glob `github.com/balinomad/llm-desk/internal/*`
  - [ ] `README.md`: update the title, the `git clone` URL, `cd llm-desk`, `go build -o llm-desk ./cmd/tui`, the `./llm-desk --data ...` usage lines, and the root of the project-structure tree
  - [ ] `docs/screen-layout.txt`: change the first word of the intro and the main-screen frame title `llm-desk` to `prompt-backlog`, and remove 6 `─` characters from that frame's top border so the box width is unchanged (the intro line grows by 6 columns; ask before rewrapping)
  - [ ] GitHub: rename the repository in Settings, then run the command below; the old URL redirects only until a new repo takes the old name

    ```bash
    git remote set-url origin https://github.com/balinomad/prompt-backlog.git
    ```

  - [ ] Verify: run `task check` and expect a clean pass
  - [ ] Verify no leftovers: run `grep -rniE 'llm-desk|llm_desk|llmdesk' . --exclude-dir=.git` and expect no output

## Other issues, ideas

- [ ] chatbot-prompt-notebook, chatbot-prompt-organizer, chatbot-session-organizer, prompt-notebook, prompt-organizer, conversation-tracker, chatbot-conversation-tracker
- [ ] `model.Registry`'s multi-result read getters (`ProjectSessions`, `AccountSessions`, `AccountProjects`, `ProjectAccounts`) return nil on stale/missing input by design, so callers don't error-check reads — revisit only if the service layer needs to distinguish "not found" from "found, zero results".
- [ ] User details should show additional information (e.g. related vendors), not the email.
- [ ] Shared data access. Enable/disable/lock/single app instance?
- [ ] frontmatters

## Public release

- [ ] Public release: relicense to MIT and reword README and GitHub metadata so the repo does not read as limit-evasion tooling or imply vendor affiliation (apply one sub-item at a time)
  - [ ] Add `LICENSE` with the standard MIT text and the line `Copyright (c) 2026 <holder>` (holder not decided, fill in before committing)
  - [ ] README, License section: replace `Proprietary. All rights reserved.` with `MIT. See [LICENSE](LICENSE).`
  - [ ] README, first paragraph: replace the whole paragraph with the text below (drops "tells you when an account is available again", inaccurate because the time is user-entered, and "switch to another account immediately", which reads as limit rotation)

    ```markdown
    `llm-desk` is a local, keyboard-driven terminal notebook for people who run several LLM conversations in parallel across vendors such as Claude, ChatGPT, and Gemini. It records a reset time that you enter for each account, and keeps a name and a queue of pending prompts for each session, so you can resume work where it stopped. It never contacts any vendor.
    ```

  - [ ] README, Features, first bullet: replace with the line below (old wording "Track rate-limit reset times" implies the tool measures limits)

    ```markdown
    - Record the reset time (`PauseUntil`) you enter for each account, independent of how many sessions run under it
    ```

  - [ ] README: add this section after `## Features`

    ```markdown
    ## Scope and Non-Goals

    - Local only: `llm-desk` makes no network requests. It does not call vendor APIs, scrape websites, or automate browsers.
    - No credentials: it stores no passwords, tokens, cookies, or API keys.
    - No sending: it never submits prompts. You copy queued prompts and paste them yourself.
    - No enforcement: pause times are values you enter. `llm-desk` does not read, measure, or enforce any vendor limit.
    - Not a workaround: it does not bypass, hide, or reduce any vendor limit or restriction.
    - Your terms apply: you are responsible for following the terms of each service you use, including its rules on accounts, account sharing, and automated access.
    ```

  - [ ] README: add this section after `## Storage`

    ```markdown
    ## Data and Privacy

    The data file is plain, unencrypted JSON. It holds user emails and display names, vendor and project names, project descriptions and instructions, and queued prompt text. It holds no credentials, and `llm-desk` sends no telemetry. Do not put secrets in prompt text or project instructions. The parent directory is created with mode `0700` when missing, and the file is written with mode `0600`.
    ```

  - [ ] README: add this section before `## License`

    ```markdown
    ## Disclaimer

    `llm-desk` is an independent project. It is not affiliated with, endorsed by, or sponsored by any LLM vendor. Claude, ChatGPT, Gemini, and other product names are trademarks of their respective owners and appear only to identify the services. The software is provided "as is" without warranty; see [LICENSE](LICENSE).
    ```

  - [ ] GitHub About text: `Offline terminal notebook for tracking LLM accounts, sessions, and queued prompts.` Topics: `go`, `tui`, `tview`, `llm`. Leave out vendor names and terms such as `rate-limit`, `bypass`, `rotation`, `multi-account`
  - [ ] Verify no network or process use before keeping the "no network requests" claim: run `grep -rnE '"(net|net/[a-z/]+|os/exec)"' --include=*.go .` and expect no output
  - [ ] Verify no leftover risky wording: run `grep -rniE 'bypass|evad|circumvent|workaround|unlimited|rotat|switch to another account' . --exclude-dir=.git` and expect hits only in the new README Scope section
  - [ ] Before the first public push, check history for real personal data, then review the output by hand

    ```bash
    git log --all --diff-filter=A --name-only --pretty=format: | sort -u
    git log --all -p | grep -E '^\+.*@[A-Za-z0-9.-]+\.[a-z]{2,}' | grep -vE 'example\.com|@x\.com' | head -20
    ```

## Skills (future — not started; existing functionality takes priority first)

- [ ] I want to attach a boolean field to vendor that indicates if it supports skills.
- [ ] We can extend the model with skills, and connect them to sessions. We also need a mechanism to register which version of the skill was installed with which session.
- Decided so far: `Skill` is an independent entity, like `Project` — named, exists on its own, then attached to sessions. No further design until this section is actually picked up.
