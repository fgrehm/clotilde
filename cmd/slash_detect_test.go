package cmd

// Internal tests for detectRename and lastCustomTitle (unexported functions).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	var content []byte
	for _, l := range lines {
		content = append(content, []byte(l+"\n")...)
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLastCustomTitle(t *testing.T) {
	t.Run("returns last custom-title value", func(t *testing.T) {
		path := writeTranscript(t,
			`{"type":"assistant","message":{"role":"assistant","content":"hello"}}`,
			`{"type":"custom-title","customTitle":"first-name","sessionId":"abc"}`,
			`{"type":"assistant","message":{"role":"assistant","content":"world"}}`,
			`{"type":"custom-title","customTitle":"final-name","sessionId":"abc"}`,
		)

		got, found := lastCustomTitle(path)
		if !found {
			t.Fatal("expected found=true")
		}
		if got != "final-name" {
			t.Fatalf("expected 'final-name', got %q", got)
		}
	})

	t.Run("returns false when no custom-title present", func(t *testing.T) {
		path := writeTranscript(t, `{"type":"assistant","message":{"role":"assistant","content":"hello"}}`)
		_, found := lastCustomTitle(path)
		if found {
			t.Fatal("expected found=false")
		}
	})

	t.Run("returns false for empty file", func(t *testing.T) {
		path := writeTranscript(t)
		_, found := lastCustomTitle(path)
		if found {
			t.Fatal("expected found=false")
		}
	})

	t.Run("returns false for non-existent file", func(t *testing.T) {
		_, found := lastCustomTitle("/does/not/exist.jsonl")
		if found {
			t.Fatal("expected found=false")
		}
	})
}

func TestDetectRename(t *testing.T) {
	setupStore := func(t *testing.T) (clotildeRoot string, store *session.FileStore) {
		t.Helper()
		tmp := t.TempDir()
		if err := config.EnsureClotildeStructure(tmp); err != nil {
			t.Fatal(err)
		}
		clotildeRoot = filepath.Join(tmp, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
		return
	}

	t.Run("renames session when custom-title differs from current name", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		sess := session.NewSession("old-session", "uuid-abc")
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}

		path := writeTranscript(t, `{"type":"custom-title","customTitle":"new-session","sessionId":"uuid-abc"}`)
		sess.Metadata.TranscriptPath = path
		if err := store.Update(sess); err != nil {
			t.Fatal(err)
		}

		detectRename(clotildeRoot, sess)

		if store.Exists("old-session") {
			t.Error("expected old-session to be gone")
		}
		if !store.Exists("new-session") {
			t.Error("expected new-session to exist")
		}
	})

	t.Run("no-op when custom-title matches current name", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		sess := session.NewSession("my-session", "uuid-xyz")
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}

		path := writeTranscript(t, `{"type":"custom-title","customTitle":"my-session","sessionId":"uuid-xyz"}`)
		sess.Metadata.TranscriptPath = path
		if err := store.Update(sess); err != nil {
			t.Fatal(err)
		}

		detectRename(clotildeRoot, sess)

		if !store.Exists("my-session") {
			t.Error("expected my-session to still exist")
		}
	})

	t.Run("no-op when custom-title is not a valid session name", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		sess := session.NewSession("valid-session", "uuid-123")
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}

		// Claude allows arbitrary display names; Clotilde requires slug format
		path := writeTranscript(t, `{"type":"custom-title","customTitle":"My Cool Feature Branch","sessionId":"uuid-123"}`)
		sess.Metadata.TranscriptPath = path
		if err := store.Update(sess); err != nil {
			t.Fatal(err)
		}

		detectRename(clotildeRoot, sess)

		if !store.Exists("valid-session") {
			t.Error("expected valid-session to still exist")
		}
	})

	t.Run("no-op when target name conflicts with existing session", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		sess := session.NewSession("source-session", "uuid-src")
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}
		existing := session.NewSession("taken-name", "uuid-taken")
		if err := store.Create(existing); err != nil {
			t.Fatal(err)
		}

		path := writeTranscript(t, `{"type":"custom-title","customTitle":"taken-name","sessionId":"uuid-src"}`)
		sess.Metadata.TranscriptPath = path
		if err := store.Update(sess); err != nil {
			t.Fatal(err)
		}

		detectRename(clotildeRoot, sess)

		if !store.Exists("source-session") {
			t.Error("expected source-session to still exist")
		}
		if !store.Exists("taken-name") {
			t.Error("expected taken-name to still exist")
		}
	})

	t.Run("no-op when transcript has no custom-title", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		sess := session.NewSession("no-rename", "uuid-nope")
		if err := store.Create(sess); err != nil {
			t.Fatal(err)
		}

		path := writeTranscript(t, `{"type":"assistant","message":{"role":"assistant","content":"hello"}}`)
		sess.Metadata.TranscriptPath = path
		if err := store.Update(sess); err != nil {
			t.Fatal(err)
		}

		detectRename(clotildeRoot, sess)

		if !store.Exists("no-rename") {
			t.Error("expected no-rename to still exist")
		}
	})
}

func TestSanitizeBranchName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"poc-test-fork-forked (Branch)", "poc-test-fork-forked"},
		{"my feature (Branch)", "my-feature"},
		{"My Cool Feature Branch (Branch)", "my-cool-feature-branch"},
		{"already-slug (Branch)", "already-slug"},
		{"no suffix at all", "no-suffix-at-all"},
		{"  spaces  (Branch)", "spaces"},
		{"(Branch)", "branch"}, // no space-prefix → suffix not stripped, sanitized as-is
		{"a (Branch)", "a"},
	}
	for _, c := range cases {
		got := sanitizeBranchName(c.input)
		if got != c.want {
			t.Errorf("sanitizeBranchName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsBranchAutoName(t *testing.T) {
	cases := []struct {
		name, parent string
		want         bool
	}{
		{"my-session-branch-1", "my-session", true},
		{"my-session-branch-99", "my-session", true},
		{"my-session-branch-0", "my-session", true},
		{"my-session-branch-", "my-session", false},
		{"my-session-branch-1x", "my-session", false},
		{"other-session-branch-1", "my-session", false},
		{"my-session", "my-session", false},
	}
	for _, c := range cases {
		got := isBranchAutoName(c.name, c.parent)
		if got != c.want {
			t.Errorf("isBranchAutoName(%q, %q) = %v, want %v", c.name, c.parent, got, c.want)
		}
	}
}

func TestDetectBranchRenames(t *testing.T) {
	setupStore := func(t *testing.T) (clotildeRoot string, store *session.FileStore) {
		t.Helper()
		tmp := t.TempDir()
		if err := config.EnsureClotildeStructure(tmp); err != nil {
			t.Fatal(err)
		}
		clotildeRoot = filepath.Join(tmp, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
		return
	}

	t.Run("renames auto-generated branch name from custom-title", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		parent := session.NewSession("poc-test-fork", "parent-uuid")
		if err := store.Create(parent); err != nil {
			t.Fatal(err)
		}

		branch := session.NewSession("poc-test-fork-branch-1", "branch-uuid")
		branch.Metadata.IsForkedSession = true
		branch.Metadata.ParentSession = "poc-test-fork"
		branchTranscript := writeTranscript(t,
			`{"type":"custom-title","customTitle":"poc-test-fork-forked (Branch)","sessionId":"branch-uuid"}`,
		)
		branch.Metadata.TranscriptPath = branchTranscript
		if err := store.Create(branch); err != nil {
			t.Fatal(err)
		}

		detectBranchRenames(clotildeRoot, parent)

		if store.Exists("poc-test-fork-branch-1") {
			t.Error("expected auto-generated name to be gone")
		}
		if !store.Exists("poc-test-fork-forked") {
			t.Error("expected sanitized branch name to exist")
		}
	})

	t.Run("skips branch without auto-generated name", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		parent := session.NewSession("my-parent", "p-uuid")
		if err := store.Create(parent); err != nil {
			t.Fatal(err)
		}

		// Branch already has a custom (non-auto-generated) name
		branch := session.NewSession("my-custom-branch", "b-uuid")
		branch.Metadata.IsForkedSession = true
		branch.Metadata.ParentSession = "my-parent"
		branchTranscript := writeTranscript(t,
			`{"type":"custom-title","customTitle":"something-else (Branch)","sessionId":"b-uuid"}`,
		)
		branch.Metadata.TranscriptPath = branchTranscript
		if err := store.Create(branch); err != nil {
			t.Fatal(err)
		}

		detectBranchRenames(clotildeRoot, parent)

		// Name should be unchanged (not auto-generated pattern)
		if !store.Exists("my-custom-branch") {
			t.Error("expected my-custom-branch to still exist")
		}
	})

	t.Run("skips rename when sanitized name is already taken", func(t *testing.T) {
		clotildeRoot, store := setupStore(t)

		parent := session.NewSession("my-parent", "p-uuid2")
		if err := store.Create(parent); err != nil {
			t.Fatal(err)
		}

		taken := session.NewSession("my-branch-name", "taken-uuid")
		if err := store.Create(taken); err != nil {
			t.Fatal(err)
		}

		branch := session.NewSession("my-parent-branch-1", "b-uuid2")
		branch.Metadata.IsForkedSession = true
		branch.Metadata.ParentSession = "my-parent"
		branchTranscript := writeTranscript(t,
			`{"type":"custom-title","customTitle":"my-branch-name (Branch)","sessionId":"b-uuid2"}`,
		)
		branch.Metadata.TranscriptPath = branchTranscript
		if err := store.Create(branch); err != nil {
			t.Fatal(err)
		}

		detectBranchRenames(clotildeRoot, parent)

		// Auto-generated name stays because desired name is taken
		if !store.Exists("my-parent-branch-1") {
			t.Error("expected my-parent-branch-1 to still exist (target name taken)")
		}
	})
}
