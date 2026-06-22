# Todo

- [x] Why does `cmd/tui/views/forms.go` have a modal function (`centeredModal`)? Shouldn't it be in `cmd/tui/views/modal.go`? Justify why it is not there.
- [x] The same quetion is true for `errorModal`
- [ ] Instead of a separate panel for unattached project, should we just have a project list with an indicator that it is unattached?
- [ ] Keymap should be rethought. `p`: project or provider? My preference is `p` for project. Then what is for provider?
- [ ] I want to attach a boolean field to provider that indicates if it supports skills.
- [ ] We can extend the model with skills, and connect them to sessions. We also need a mechanism to register which version of the skill was installed with which session.
- [ ] Unattached is questionable, projects should be used for both attached and unattached.
- [ ] Need an numbered list to summarize what to do next (per session). This can be more complex (type: prompt, todo) for each list item.
- [ ] Add CI: `.golangci.yml` and `.github/workflows/go.yml`
- [ ] `model.Registry`'s multi-result read getters (`ProjectSessions`, `AccountSessions`, `AccountProjects`, `ProjectAccounts`) return nil on stale/missing input by design, so callers don't error-check reads — revisit only if the service layer needs to distinguish "not found" from "found, zero results."
