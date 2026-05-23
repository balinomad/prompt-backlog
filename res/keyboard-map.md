# Keyboard map

| Key      | Context                           | Action                      |
| -------- | --------------------------------- | --------------------------- |
| `↑↓`     | any list                          | navigate                    |
| `Tab`    | anywhere                          | rotate focus between panels |
| `Enter`  | session list                      | expand → focus projects     |
| `n`      | sessions / projects / standalone  | new item form               |
| `e`      | any selected item                 | edit form                   |
| `d`      | any selected item                 | delete (confirm modal)      |
| `m`      | project or last-question selected | toggle mid-answer           |
| `Ctrl+S` | form                              | save                        |
| `Esc`    | form / modal                      | cancel / close              |
| `s`      | sessions panel                    | switch to standalone screen |
| `?`      | anywhere                          | help overlay                |
| `q`      | anywhere                          | quit (confirm if unsaved)   |

**Color semantics** (works in 16-color terminals):

- `mid-answer = true` → item label in **yellow**
- selected item → **reverse video** (default tview behaviour)
- session with `next_prompt_at` in the past → time shown in **red**
- saved status indicator in footer: `saved ✓` (green) / `unsaved ●` (yellow)
