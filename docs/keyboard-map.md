# Keyboard map

| Key         | Context                             | Action                                      |
| ----------- | ----------------------------------- | ------------------------------------------- |
| `↑↓`        | any list                            | navigate                                    |
| `Tab`       | accounts list                       | switch to session detail                    |
| `Tab`       | session detail                      | switch to accounts list                     |
| `Tab`       | projects screen, project list       | switch to account list                      |
| `Tab`       | projects screen, account list       | switch to project list                      |
| `Tab`       | form                                | next field                                  |
| `Shift+Tab` | two-pane screens                    | switch focus backward                       |
| `Insert`    | accounts list                       | add account form                            |
| `Insert`    | providers screen                    | add provider form                           |
| `Insert`    | projects screen, project list       | add project form                            |
| `Insert`    | projects screen, account list       | add account to project                      |
| `Insert`    | session detail                      | add session form                            |
| `Insert`    | queue editor                        | add prompt form                             |
| `Enter`     | accounts list, selected account     | edit account (pause state)                  |
| `Enter`     | providers screen, selected provider | edit provider form                          |
| `Enter`     | projects screen, selected project   | edit project form                           |
| `Enter`     | session detail, selected session    | edit session form                           |
| `Enter`     | queue editor, selected prompt       | edit prompt form                            |
| `Del`       | accounts list, selected account     | delete account (confirm modal)              |
| `Del`       | providers screen, selected provider | delete provider (confirm modal)             |
| `Del`       | projects screen, selected project   | delete project (confirm modal)              |
| `Del`       | projects screen, account list       | delete account from project (confirm modal) |
| `Del`       | session detail, selected session    | delete session (confirm modal)              |
| `Del`       | queue editor, selected prompt       | delete prompt (confirm modal)               |
| `Space`     | session detail, selected session    | toggle "continue" prompt                    |
| `Esc`       | form / modal                        | cancel / close                              |
| `Esc`       | providers screen                    | return to main screen                       |
| `Esc`       | projects screen, project list       | return to main screen                       |
| `Esc`       | projects screen, account list       | return to project list                      |
| `Esc`       | queue editor                        | close queue editor                          |
| `Esc`       | help overlay                        | close help overlay                          |
| `r`         | accounts list                       | switch to providers screen                  |
| `p`         | accounts list                       | switch to projects screen                   |
| `u`         | session detail, selected session    | open queue editor                           |
| `Ctrl+↑`    | queue editor, selected prompt       | move prompt up                              |
| `Ctrl+↓`    | queue editor, selected prompt       | move prompt down                            |
| `?`         | main, providers, projects, queue    | help overlay                                |
| `q`         | main, providers, projects, queue    | quit                                        |

`Tab` and `Shift+Tab` are ignored on the single-pane providers screen and queue editor. The projects screen and account list use `Tab` to switch panes; `Esc` returns from the account list to the project list.

**Color semantics** (works in 16-color terminals):

- `mid-answer = true` → item label in **yellow**
- selected item → **reverse video** (default tview behaviour)
- session with a pending "continue" prompt (session detail list) → item shown in **yellow**, marked `↻`
- prompt that spawns a new session (queue editor) → marked `→ new session` in **cyan**
- account pause status, accounts list: `PauseUntil` in the future (still unavailable) → shown in **red**; in the past (available again) → shown in **green**; never paused → nothing shown
- provider capability badges (providers screen): `projects` → **green**; `deep-research` → **cyan**
- boolean capability display (`yes` / `no`): `yes` → **green**; `no` → **dim**
- saved status indicator in footer: `saved ✓` (**green**) on a successful save; save/validation errors shown in **red**
