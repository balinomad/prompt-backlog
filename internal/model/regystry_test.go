package model_test

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// seedRegistry returns a populated registry for use across multiple tests.
//
//	Providers: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Sessions:  alice → claude, bob → chatgpt
//	Projects:  "Alpha" (alice/claude), "Beta" (alice/claude), "Orphan" (unattached)
func seedRegistry(t *testing.T) model.Registry {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("bob@example.com"),
		reg.ProviderByName("chatgpt"),
	)))
	must(t, reg.AddProject(model.NewSessionProject("Alpha",
		reg.SessionAt("alice@example.com", "claude"),
	)))
	must(t, reg.AddProject(model.NewSessionProject("Beta",
		reg.SessionAt("alice@example.com", "claude"),
	)))
	must(t, reg.AddProject(model.NewProject("Orphan")))
	return reg
}

// ── Provider CRUD ─────────────────────────────────────────────────────────────

func TestAddProvider_Success(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	if reg.ProviderByName("claude") == nil {
		t.Error("provider not found after add")
	}
}

func TestAddProvider_NilPointer(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddProvider(nil); err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestAddProvider_EmptyName(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddProvider(model.NewProvider("", false, false)); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAddProvider_Duplicate(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	if err := reg.AddProvider(model.NewProvider("claude", true, false)); err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestProviderByName_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if reg.ProviderByName("ghost") != nil {
		t.Error("expected nil for unknown provider")
	}
}

func TestAllProviders_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.AllProviders() == nil {
		t.Error("AllProviders: expected non-nil empty slice")
	}
}

func TestAllProviders_ReturnsAll(t *testing.T) {
	reg := seedRegistry(t)
	if n := len(reg.AllProviders()); n != 2 {
		t.Errorf("AllProviders: expected 2, got %d", n)
	}
}

func TestRemoveProvider_Success(t *testing.T) {
	reg := seedRegistry(t)
	// Remove sessions for chatgpt first so provider can be removed.
	must(t, reg.RemoveSession("bob@example.com", "chatgpt"))
	must(t, reg.RemoveProvider("chatgpt"))
	if reg.ProviderByName("chatgpt") != nil {
		t.Error("provider still found after removal")
	}
}

func TestRemoveProvider_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.RemoveProvider("ghost"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRemoveProvider_HasActiveSessions(t *testing.T) {
	reg := seedRegistry(t)
	// "claude" has alice's session; removal must be blocked.
	if err := reg.RemoveProvider("claude"); err == nil {
		t.Error("expected error: provider has active sessions")
	}
}

// ── User CRUD ─────────────────────────────────────────────────────────────────

func TestAddUser_Success(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	if reg.UserByEmail("alice@example.com") == nil {
		t.Error("user not found after add")
	}
}

func TestAddUser_NilPointer(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddUser(nil); err == nil {
		t.Error("expected error for nil user")
	}
}

func TestAddUser_EmptyEmail(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddUser(model.NewUser("", "Alice")); err == nil {
		t.Error("expected error for empty email")
	}
}

func TestAddUser_Duplicate(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	if err := reg.AddUser(model.NewUser("alice@example.com", "Alice2")); err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestUserByEmail_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if reg.UserByEmail("ghost@example.com") != nil {
		t.Error("expected nil for unknown user")
	}
}

func TestAllUsers_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.AllUsers() == nil {
		t.Error("AllUsers: expected non-nil empty slice")
	}
}

func TestAllUsers_ReturnsAll(t *testing.T) {
	reg := seedRegistry(t)
	if n := len(reg.AllUsers()); n != 2 {
		t.Errorf("AllUsers: expected 2, got %d", n)
	}
}

func TestRemoveUser_Success(t *testing.T) {
	reg := seedRegistry(t)
	// Remove bob's session first.
	must(t, reg.RemoveSession("bob@example.com", "chatgpt"))
	must(t, reg.RemoveUser("bob@example.com"))
	if reg.UserByEmail("bob@example.com") != nil {
		t.Error("user still found after removal")
	}
}

func TestRemoveUser_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.RemoveUser("ghost@example.com"); err == nil {
		t.Error("expected error for unknown user")
	}
}

func TestRemoveUser_HasActiveSessions(t *testing.T) {
	reg := seedRegistry(t)
	// alice has an active session; removal must be blocked.
	if err := reg.RemoveUser("alice@example.com"); err == nil {
		t.Error("expected error: user has active sessions")
	}
}

// ── Session CRUD ──────────────────────────────────────────────────────────────

func TestAddSession_Success(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	if s == nil {
		t.Error("session not found after add")
	}
}

func TestAddSession_NilPointer(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddSession(nil); err == nil {
		t.Error("expected error for nil session")
	}
}

func TestAddSession_NilUser(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	s := model.NewSession(nil, reg.ProviderByName("claude"))
	if err := reg.AddSession(s); err == nil {
		t.Error("expected error for nil user")
	}
}

func TestAddSession_NilProvider(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	s := model.NewSession(reg.UserByEmail("alice@example.com"), nil)
	if err := reg.AddSession(s); err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestAddSession_UserNotInRegistry(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	// User constructed but never added to registry.
	outsideUser := model.NewUser("outside@example.com", "Outside")
	s := model.NewSession(outsideUser, reg.ProviderByName("claude"))
	if err := reg.AddSession(s); err == nil {
		t.Error("expected error: user not registered in this registry")
	}
}

func TestAddSession_ProviderNotInRegistry(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	// Provider constructed but never added to registry.
	outsideProvider := model.NewProvider("outside", true, false)
	s := model.NewSession(reg.UserByEmail("alice@example.com"), outsideProvider)
	if err := reg.AddSession(s); err == nil {
		t.Error("expected error: provider not registered in this registry")
	}
}

func TestAddSession_Duplicate(t *testing.T) {
	reg := seedRegistry(t)
	// alice/claude already exists.
	s := model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)
	if err := reg.AddSession(s); err == nil {
		t.Error("expected error for duplicate session")
	}
}

func TestAddSession_PopulatesBothIndexes(t *testing.T) {
	reg := seedRegistry(t)
	// Verify session is reachable via both SessionsByUser and SessionsByProvider.
	byUser := reg.SessionsByUser("alice@example.com")
	byProvider := reg.SessionsByProvider("claude")
	if len(byUser) == 0 {
		t.Error("session not found via user index")
	}
	if len(byProvider) == 0 {
		t.Error("session not found via provider index")
	}
	// Both indexes must return the same pointer.
	if byUser[0] != byProvider[0] {
		t.Error("user and provider indexes return different pointers")
	}
}

func TestSessionAt_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if reg.SessionAt("ghost@example.com", "claude") != nil {
		t.Error("expected nil for unknown session")
	}
}

func TestAllSessions_NoDuplicates(t *testing.T) {
	reg := seedRegistry(t)
	// Dual-index must not cause duplicates in AllSessions.
	if n := len(reg.AllSessions()); n != 2 {
		t.Errorf("AllSessions: expected 2, got %d", n)
	}
}

func TestAllSessions_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.AllSessions() == nil {
		t.Error("AllSessions: expected non-nil empty slice")
	}
}

func TestSessionsByUser_ReturnsOnlyUserSessions(t *testing.T) {
	reg := seedRegistry(t)
	aliceSessions := reg.SessionsByUser("alice@example.com")
	if len(aliceSessions) != 1 {
		t.Errorf("expected 1 session for alice, got %d", len(aliceSessions))
	}
}

func TestSessionsByUser_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.SessionsByUser("nobody@example.com") == nil {
		t.Error("SessionsByUser: expected non-nil empty slice")
	}
}

func TestSessionsByProvider_ReturnsOnlyProviderSessions(t *testing.T) {
	reg := seedRegistry(t)
	claudeSessions := reg.SessionsByProvider("claude")
	if len(claudeSessions) != 1 {
		t.Errorf("expected 1 session for claude, got %d", len(claudeSessions))
	}
}

func TestSessionsByProvider_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.SessionsByProvider("nobody") == nil {
		t.Error("SessionsByProvider: expected non-nil empty slice")
	}
}

func TestRemoveSession_Success(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveSession("alice@example.com", "claude"))
	if reg.SessionAt("alice@example.com", "claude") != nil {
		t.Error("session still found after removal")
	}
}

func TestRemoveSession_CleansUserIndex(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveSession("alice@example.com", "claude"))
	if n := len(reg.SessionsByUser("alice@example.com")); n != 0 {
		t.Errorf("user index: expected 0 sessions after removal, got %d", n)
	}
}

func TestRemoveSession_CleansProviderIndex(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveSession("alice@example.com", "claude"))
	if n := len(reg.SessionsByProvider("claude")); n != 0 {
		t.Errorf("provider index: expected 0 sessions after removal, got %d", n)
	}
}

func TestRemoveSession_DetachesProjects(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveSession("alice@example.com", "claude"))
	// Alpha and Beta were attached to alice/claude; they must now be unattached.
	for _, name := range []string{"Alpha", "Beta"} {
		p := reg.ProjectByName(name)
		if p == nil {
			t.Fatalf("project %q should still exist after session removal", name)
		}
		if !p.IsUnattached() {
			t.Errorf("project %q should be unattached after session removal", name)
		}
	}
}

func TestRemoveSession_PreservesUnrelatedProjects(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveSession("alice@example.com", "claude"))
	// Orphan was already unattached and must be unaffected.
	if reg.ProjectByName("Orphan") == nil {
		t.Error("Orphan project should still exist")
	}
}

func TestRemoveSession_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.RemoveSession("ghost@example.com", "claude"); err == nil {
		t.Error("expected error for unknown session")
	}
}

// ── Project CRUD ──────────────────────────────────────────────────────────────

func TestAddProject_StandaloneSuccess(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProject(model.NewProject("Orphan")))
	if reg.ProjectByName("Orphan") == nil {
		t.Error("project not found after add")
	}
}

func TestAddProject_SessionSuccess(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	must(t, reg.AddProject(model.NewSessionProject("Gamma", s)))
	if reg.ProjectByName("Gamma") == nil {
		t.Error("project not found after add")
	}
}

func TestAddProject_NilPointer(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddProject(nil); err == nil {
		t.Error("expected error for nil project")
	}
}

func TestAddProject_EmptyName(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.AddProject(model.NewProject("")); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAddProject_Duplicate(t *testing.T) {
	reg := model.NewRegistry()
	must(t, reg.AddProject(model.NewProject("Alpha")))
	if err := reg.AddProject(model.NewProject("Alpha")); err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestAddProject_ProviderDoesNotSupportProjects(t *testing.T) {
	reg := seedRegistry(t)
	// bob uses chatgpt which has HasProjects=false.
	s := reg.SessionAt("bob@example.com", "chatgpt")
	p := model.NewSessionProject("X", s)
	if err := reg.AddProject(p); err == nil {
		t.Error("expected error: provider does not support projects")
	}
}

func TestProjectByName_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if reg.ProjectByName("ghost") != nil {
		t.Error("expected nil for unknown project")
	}
}

func TestAllProjects_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.AllProjects() == nil {
		t.Error("AllProjects: expected non-nil empty slice")
	}
}

func TestAllProjects_ReturnsAll(t *testing.T) {
	reg := seedRegistry(t)
	if n := len(reg.AllProjects()); n != 3 {
		t.Errorf("AllProjects: expected 3, got %d", n)
	}
}

func TestUnattachedProjects_ReturnsOnlyUnattached(t *testing.T) {
	reg := seedRegistry(t)
	unattached := reg.UnattachedProjects()
	if len(unattached) != 1 {
		t.Fatalf("expected 1 unattached project, got %d", len(unattached))
	}
	if unattached[0].Name() != "Orphan" {
		t.Errorf("expected Orphan, got %q", unattached[0].Name())
	}
}

func TestUnattachedProjects_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.UnattachedProjects() == nil {
		t.Error("UnattachedProjects: expected non-nil empty slice")
	}
}

func TestProjectsBySession_ReturnsAttachedProjects(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	projects := reg.ProjectsBySession(s)
	if len(projects) != 2 {
		t.Errorf("expected 2 projects for alice/claude, got %d", len(projects))
	}
}

func TestProjectsBySession_ExcludesUnattached(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	for _, p := range reg.ProjectsBySession(s) {
		if p.Name() == "Orphan" {
			t.Error("Orphan should not appear in session projects")
		}
	}
}

func TestProjectsBySession_NeverNil(t *testing.T) {
	reg := model.NewRegistry()
	if reg.ProjectsBySession(nil) == nil {
		t.Error("ProjectsBySession: expected non-nil empty slice")
	}
}

func TestAttachProject_Success(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	must(t, reg.AttachProject("Orphan", s))
	if reg.ProjectByName("Orphan").Session() != s {
		t.Error("Orphan should be attached to alice/claude after attach")
	}
}

func TestAttachProject_NotFound(t *testing.T) {
	reg := seedRegistry(t)
	s := reg.SessionAt("alice@example.com", "claude")
	if err := reg.AttachProject("ghost", s); err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestAttachProject_NilSession(t *testing.T) {
	reg := seedRegistry(t)
	if err := reg.AttachProject("Orphan", nil); err == nil {
		t.Error("expected error for nil session")
	}
}

func TestAttachProject_ProviderDoesNotSupportProjects(t *testing.T) {
	reg := seedRegistry(t)
	// bob/chatgpt does not support projects.
	s := reg.SessionAt("bob@example.com", "chatgpt")
	if err := reg.AttachProject("Orphan", s); err == nil {
		t.Error("expected error: provider does not support projects")
	}
}

func TestDetachProject_Success(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.DetachProject("Alpha"))
	if !reg.ProjectByName("Alpha").IsUnattached() {
		t.Error("Alpha should be unattached after detach")
	}
}

func TestDetachProject_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.DetachProject("ghost"); err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestRemoveProject_Success(t *testing.T) {
	reg := seedRegistry(t)
	must(t, reg.RemoveProject("Orphan"))
	if reg.ProjectByName("Orphan") != nil {
		t.Error("project still found after removal")
	}
}

func TestRemoveProject_NotFound(t *testing.T) {
	reg := model.NewRegistry()
	if err := reg.RemoveProject("ghost"); err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestRemoveProject_ClearsLastQuestionPointer(t *testing.T) {
	reg := seedRegistry(t)
	// Attach a LastQuestion referencing Alpha to alice's session.
	s := reg.SessionAt("alice@example.com", "claude")
	s.LastQuestion = model.NewLastQuestion(reg.ProjectByName("Alpha"))
	// Remove Alpha — LastQuestion.Project() must become nil.
	must(t, reg.RemoveProject("Alpha"))
	if s.LastQuestion.Project() != nil {
		t.Error("LastQuestion.Project should be nil after project removal")
	}
}

func TestRemoveProject_DoesNotAffectOtherLastQuestions(t *testing.T) {
	reg := seedRegistry(t)
	// Attach LastQuestion referencing Beta to alice's session.
	s := reg.SessionAt("alice@example.com", "claude")
	s.LastQuestion = model.NewLastQuestion(reg.ProjectByName("Beta"))
	// Remove Alpha (different project) — LastQuestion must be unaffected.
	must(t, reg.RemoveProject("Alpha"))
	if s.LastQuestion.Project() == nil {
		t.Error("LastQuestion.Project should still point to Beta")
	}
}
