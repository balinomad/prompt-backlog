# Keyboard map

| Key         | Context                             | Action                          |
| ----------- | ----------------------------------- | ------------------------------- |
| `↑↓`        | any list                            | navigate                        |
| `Tab`       | anywhere                            | rotate focus forward            |
| `Shift+Tab` | anywhere                            | rotate focus backward           |
| `n`         | accounts list                       | new account form                |
| `n`         | providers screen                    | new provider form               |
| `n`         | projects screen                     | new project form                |
| `n`         | session detail                      | new session form                |
| `n`         | queue editor                        | new prompt form                 |
| `Enter`     | accounts list, selected account     | edit account (pause state)      |
| `Enter`     | providers screen, selected provider | edit provider form              |
| `Enter`     | projects screen, selected project   | edit project form               |
| `e`         | session detail, selected session    | edit session form               |
| `Enter`     | queue editor, selected prompt       | edit prompt form                |
| `F2`        | accounts list, selected account     | rename user                     |
| `Del`       | accounts list, selected account     | delete account (confirm modal)  |
| `Del`       | providers screen, selected provider | delete provider (confirm modal) |
| `Del`       | projects screen, selected project   | delete project (confirm modal)  |
| `Del`       | session detail, selected session    | end session (confirm modal)     |
| `Del`       | queue editor, selected prompt       | delete prompt (confirm modal)   |
| `Space`     | session detail, selected session    | toggle "continue" prompt        |
| `Ctrl+S`    | form                                | save                            |
| `Esc`       | form / modal                        | cancel / close                  |
| `Esc`       | providers screen, projects screen   | return to main screen           |
| `Esc`       | queue editor                        | close queue editor              |
| `r`         | accounts list                       | switch to providers screen      |
| `p`         | accounts list                       | switch to projects screen       |
| `u`         | session detail, selected session    | open queue editor               |
| `+`         | projects screen, selected project   | grant to an account             |
| `-`         | projects screen, selected project   | revoke from an account          |
| `K`         | queue editor, selected prompt       | move prompt up                  |
| `J`         | queue editor, selected prompt       | move prompt down                |
| `h`         | anywhere                            | help overlay                    |
| `q`         | anywhere                            | quit                            |

**Color semantics** (works in 16-color terminals):

- `mid-answer = true` → item label in **yellow**
- selected item → **reverse video** (default tview behaviour)
- session with a pending "continue" prompt (session detail list) → item shown in **yellow**, marked `↻`
- prompt that spawns a new session (queue editor) → marked `→ new session` in **cyan**
- account pause status, both the accounts list and session detail's account-info panel: `PauseUntil` in the future (still unavailable) → shown in **red**; in the past (available again) → shown in **green**; never paused → nothing shown
- provider capability badges (providers screen): `projects` → **green**; `deep-research` → **cyan**
- boolean capability display (`yes` / `no`): `yes` → **green**; `no` → **dim**
- project with zero granted accounts (projects screen) → row shown **dim**, flagged `unattached`
- saved status indicator in footer: `saved ✓` (**green**) on a successful save; save/validation errors shown in **red**
