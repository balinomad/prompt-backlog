# Todo

## Errors

## Other issues, ideas

- [ ] `model.Registry`'s multi-result read getters (`ProjectSessions`, `AccountSessions`, `AccountProjects`, `ProjectAccounts`) return nil on stale/missing input by design, so callers don't error-check reads — revisit only if the service layer needs to distinguish "not found" from "found, zero results".
- [ ] User details should show additional information (e.g. related vendors), not the email.
- [ ] Shared data access. Enable/disable/lock/single app instance?
- [ ] frontmatters
