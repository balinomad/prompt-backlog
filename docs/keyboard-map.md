# Keyboard map

| Key         | Context                                  | Action                                      |
| ----------- | ---------------------------------------- | ------------------------------------------- |
| `↑↓`        | any list                                 | navigate                                    |
| `Tab`       | accounts list                            | switch to sessions                          |
| `Tab`       | sessions pane                            | switch to notes                             |
| `Tab`       | notes pane                               | switch to accounts                          |
| `Tab`       | projects screen, projects list           | switch to project accounts                  |
| `Tab`       | projects screen, project accounts list   | switch to project notes                     |
| `Tab`       | projects screen, project notes list      | switch to projects                          |
| `Tab`       | form                                     | next field                                  |
| `Shift+Tab` | main screen                              | switch focus backward                       |
| `Shift+Tab` | projects screen                          | switch focus backward                       |
| `Insert`    | accounts list                            | add account form                            |
| `Insert`    | vendors screen                           | add vendor form                             |
| `Insert`    | users screen                             | add user form                               |
| `Insert`    | projects screen, projects list           | add project form                            |
| `Insert`    | projects screen, project accounts list   | add account to project                      |
| `Insert`    | projects screen, project notes list      | add note to project                         |
| `Insert`    | sessions pane                            | start session form                          |
| `Insert`    | prompt form, prompt list                 | add prompt to session                       |
| `Insert`    | notes pane                               | add note form                               |
| `Enter`     | accounts list, selected account          | pause account form                          |
| `Enter`     | vendors screen, selected vendor          | edit vendor form                            |
| `Enter`     | users screen, selected user              | edit user form                              |
| `Enter`     | projects screen, selected project        | edit project form                           |
| `Enter`     | projects screen, project notes list      | edit note form                              |
| `Enter`     | sessions pane, selected session          | open prompt form                            |
| `Enter`     | prompt form, prompt list                 | focus prompt editor                         |
| `Enter`     | notes pane, selected note                | edit note form                              |
| `Del`       | accounts list, selected account          | delete account (confirm modal)              |
| `Del`       | vendors screen, selected vendor          | delete vendor (confirm modal)               |
| `Del`       | users screen, selected user              | delete user (confirm modal)                 |
| `Del`       | projects screen, selected project        | delete project (confirm modal)              |
| `Del`       | projects screen, project accounts list   | delete account from project (confirm modal) |
| `Del`       | projects screen, project notes list      | delete note (confirm modal)                 |
| `Del`       | sessions pane, selected session          | delete session (confirm modal)              |
| `Del`       | prompt form, prompt list                 | remove prompt from draft (confirm modal)    |
| `Del`       | notes pane, selected note                | delete note (confirm modal)                 |
| `s`         | notes pane                               | start session from selected note            |
| `f`         | accounts list, sessions pane, notes pane | advance project filter                      |
| `Esc`       | form / modal                             | cancel / close top modal                    |
| `Esc`       | vendors screen                           | return to main screen                       |
| `Esc`       | users screen                             | return to main screen                       |
| `Esc`       | projects screen, any list                | return to main screen                       |
| `Esc`       | help overlay                             | close help overlay                          |
| `v`         | any screen                               | switch to vendors screen                    |
| `p`         | any screen                               | switch to projects screen                   |
| `u`         | any screen                               | switch to users screen                      |
| `?`         | main, vendors, users, projects           | help overlay                                |
| `q`         | main, vendors, users, projects           | quit                                        |

`Tab` and `Shift+Tab` are ignored on the single-pane vendors and users screens. The main screen and the projects screen use `Tab` to switch panes; `Esc` from any projects list returns to the main screen.

The `v`, `p`, and `u` screen-switch keys are ignored while a form, help overlay, or confirmation modal has focus.

The Sessions pane shows one row per session: project, session name, prompt count, and a status marker. The project column is hidden when no visible session has a project. Prompt count is the number after the session name.

The prompt form shows the session's prompt queue above the prompt editor. When the queue has focus, normal list keys navigate it, `Insert` adds a prompt, and `Del` removes the selected prompt after confirmation. The list `Del` change is persisted only on Save. The `Delete` button removes the current prompt, confirms, and saves immediately.

When a modal opens another modal, the parent remains visible with an inactive border. `Esc` closes one modal level and returns to the parent.

## Mouse

Mouse reporting must be enabled by the terminal. Clicking a row selects it; the wheel scrolls lists. Double-click runs the same action as Enter where a row has one.

| Context                             | Double-click action       |
| ----------------------------------- | ------------------------- |
| Accounts list                       | Open the pause form       |
| Vendors screen                      | Edit the selected vendor  |
| Users screen                        | Edit the selected user    |
| Projects screen, projects list      | Edit the selected project |
| Projects screen, project notes list | Edit the selected note    |
| Sessions pane, selected session     | Edit the prompt           |
| Prompt form, prompt list            | Focus the prompt editor   |
| Notes pane                          | Edit the selected note    |

The Projects screen's granted-accounts list has no double-click action. A double-click on a list border is ignored.

Terminal mouse reporting varies. Some terminals or multiplexers intercept clicks; Shift+click is often needed for native text selection.

**Colour semantics** (works in 16-color terminals):

- selected item: **reverse video** (default tview behaviour)
- session status: **yellow** `✓` for needs closing, **gray** `✓` for closed, no marker for active
- account pause status, accounts list: `PauseUntil` in the future (still unavailable) shown in **red**; in the past (available again) shown in **green**; never paused shows nothing
- vendor capability badges (vendors screen): `projects` shown in **green**; `deep-research` shown in **cyan**
- boolean capability display (`yes` / `no`): `yes` shown in **green**; `no` shown in **dim**
- saved status indicator in footer: `saved ✓` (**green**) on a successful save; save/validation errors shown in **red**
