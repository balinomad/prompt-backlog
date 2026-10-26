# Keyboard map

| Key         | Context                                  | Action                                      |
| ----------- | ---------------------------------------- | ------------------------------------------- |
| `↑↓`        | any list                                 | navigate                                    |
| `Tab`       | accounts list                            | switch to session detail                    |
| `Tab`       | session detail                           | switch to accounts list                     |
| `Tab`       | projects screen, project list            | switch to account list                      |
| `Tab`       | projects screen, account list            | switch to project list                      |
| `Tab`       | form                                     | next field                                  |
| `Shift+Tab` | two-pane screens                         | switch focus backward                       |
| `Insert`    | accounts list                            | add account form                            |
| `Insert`    | vendors screen                           | add vendor form                             |
| `Insert`    | users screen                             | add user form                               |
| `Insert`    | projects screen, project list            | add project form                            |
| `Insert`    | projects screen, account list            | add account to project                      |
| `Insert`    | session detail                           | add session form                            |
| `Insert`    | messages editor                          | add message form                            |
| `Enter`     | accounts list, selected account          | pause account form                          |
| `Enter`     | vendors screen, selected vendor          | edit vendor form                            |
| `Enter`     | users screen, selected user              | edit user form                              |
| `Enter`     | projects screen, selected project        | edit project form                           |
| `Enter`     | session detail, selected session         | edit session form                           |
| `Enter`     | messages editor, selected message        | edit message form                           |
| `Del`       | accounts list, selected account          | delete account (confirm modal)              |
| `Del`       | vendors screen, selected vendor          | delete vendor (confirm modal)               |
| `Del`       | users screen, selected user              | delete user (confirm modal)                 |
| `Del`       | projects screen, selected project        | delete project (confirm modal)              |
| `Del`       | projects screen, account list            | delete account from project (confirm modal) |
| `Del`       | session detail, selected session         | delete session (confirm modal)              |
| `Del`       | messages editor, selected message        | delete message (confirm modal)              |
| `Space`     | session detail, selected session         | toggle "continue" message                   |
| `Esc`       | form / modal                             | cancel / close                              |
| `Esc`       | vendors screen                           | return to main screen                       |
| `Esc`       | users screen                             | return to main screen                       |
| `Esc`       | projects screen, project list            | return to main screen                       |
| `Esc`       | projects screen, account list            | return to project list                      |
| `Esc`       | messages editor                          | close messages editor                       |
| `Esc`       | help overlay                             | close help overlay                          |
| `v`         | accounts list                            | switch to vendors screen                    |
| `p`         | accounts list                            | switch to projects screen                   |
| `u`         | accounts list                            | switch to users screen                      |
| `m`         | session detail, selected session         | open messages editor                        |
| `Ctrl+↑`    | messages editor, selected message        | move message up                             |
| `Ctrl+↓`    | messages editor, selected message        | move message down                           |
| `?`         | main, vendors, users, projects, messages | help overlay                                |
| `q`         | main, vendors, users, projects, messages | quit                                        |

`Tab` and `Shift+Tab` are ignored on the single-pane vendors screen, users screen, and messages editor. The projects screen and account list use `Tab` to switch panes; `Esc` returns from the account list to the project list.

**Colour semantics** (works in 16-color terminals):

- selected item → **reverse video** (default tview behaviour)
- session with a pending "continue" message (session detail list) → item shown in **yellow**, marked `↻`
- message that spawns a new session (messages editor) → marked `→ new session` in **cyan**
- account pause status, accounts list: `PauseUntil` in the future (still unavailable) → shown in **red**; in the past (available again) → shown in **green**; never paused → nothing shown
- vendor capability badges (vendors screen): `projects` → **green**; `deep-research` → **cyan**
- boolean capability display (`yes` / `no`): `yes` → **green**; `no` → **dim**
- saved status indicator in footer: `saved ✓` (**green**) on a successful save; save/validation errors shown in **red**
