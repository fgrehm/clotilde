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
