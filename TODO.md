# Todo

- [ ] Need a numbered list to summarize what to do next (per session). This can be more complex (type: prompt, todo) for each list item.
- [ ] `model.Registry`'s multi-result read getters (`ProjectSessions`, `AccountSessions`, `AccountProjects`, `ProjectAccounts`) return nil on stale/missing input by design, so callers don't error-check reads — revisit only if the service layer needs to distinguish "not found" from "found, zero results".
- [ ] Black box test everywhere.
- [ ] Minimum width and height? Scrollbars - where?
- [ ] User details should show additional information (e.g. related vendors), not the email.
- [ ] Build queue from ground up. Needs thinking.

## Skills (future — not started; existing functionality takes priority first)

- [ ] I want to attach a boolean field to vendor that indicates if it supports skills.
- [ ] We can extend the model with skills, and connect them to sessions. We also need a mechanism to register which version of the skill was installed with which session.
- Decided so far: `Skill` is an independent entity, like `Project` — named, exists on its own, then attached to sessions. No further design until this section is actually picked up.
