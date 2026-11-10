# Todo

## Errors

- [ ] In modals, where Clear/Copy/Paste are on the left side, the correct order should be Paste/Clear/Copy, so for a pressing Shift+Tab once, the control goes from the multiline text to Copy.
- [ ] Time field in Pause model: datetime doesn't fit and shifts out the left character when the cursor moves. At the same time there is unused space on the right.

## Other issues, ideas

- [ ] `model.Registry`'s multi-result read getters (`ProjectSessions`, `AccountSessions`, `AccountProjects`, `ProjectAccounts`) return nil on stale/missing input by design, so callers don't error-check reads — revisit only if the service layer needs to distinguish "not found" from "found, zero results".
- [ ] User details should show additional information (e.g. related vendors), not the email.
- [ ] Shared data access. Enable/disable/lock/single app instance?
- [ ] frontmatters
