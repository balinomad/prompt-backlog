## How Cloude Can Help

_Cloude_ can generate **correct, complete, idiomatic Go** for every phase above. What that means practically:

**What _Cloude_ does:** generates each file in full — types, tests, tview wiring, error handling — ready to compile. _Cloude_ doesn't produce pseudo-code or sketches unless the user asks for them.

**What the user does:** reviews the generated code, runs tests, makes judgment calls on things only the user knows (keybinding preferences, exact field labels, data file path), and catches anything _Cloude_ get subtly wrong about the specific intentions.

**Realistic split with _Cloude_'s help:**

| Phase                | Solo | With _Cloude_               |
| -------------------- | ---- | --------------------------- |
| Model + store        | 3h   | 30 min (review + `go test`) |
| TUI scaffold + views | 9h   | 2–3h (wire, run, tweak)     |
| Polish + edge cases  | 3h   | 1h                          |
| **Total**            | ~15h | **~4h**                     |

**Ground rules for working with _Cloude_ efficiently:**

- **One package at a time**, in dependency order (model → store → tui views)
- After each package: the user compiles, runs tests, confirms it matches intent — then move on
- The user tells _Cloude_ when something doesn't match their mental model; _Cloude_ fixes the whole file, not a patch on a patch
- _Cloude_ will flag every place where it makes an assumption about a preference
