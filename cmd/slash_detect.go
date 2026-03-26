package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

// detectRename checks whether the user ran /rename inside Claude and, if so,
// renames the Clotilde session to match. Called after invokeInteractive returns.
func detectRename(clotildeRoot string, sess *session.Session) {
	store := session.NewFileStore(clotildeRoot)

	// Reload to pick up hook-updated metadata (transcript path written by SessionStart hook).
	current, err := store.Get(sess.Name)
	if err != nil {
		return // session no longer exists (already cleaned up)
	}

	transcriptPath := current.Metadata.TranscriptPath
	if transcriptPath == "" {
		homeDir, hErr := util.HomeDir()
		if hErr != nil {
			return
		}
		transcriptPath = claude.TranscriptPath(homeDir, clotildeRoot, current.Metadata.SessionID)
	}

	if !util.FileExists(transcriptPath) {
		return
	}

	newName, found := lastCustomTitle(transcriptPath)
	if !found || newName == sess.Name {
		return
	}

	if err := session.ValidateName(newName); err != nil {
		fmt.Fprintf(os.Stderr, "clotilde: /rename detected but '%s' is not a valid session name: %v\n", newName, err)
		return
	}

	if store.Exists(newName) {
		fmt.Fprintf(os.Stderr, "clotilde: /rename detected but session '%s' already exists\n", newName)
		return
	}

	if err := store.Rename(sess.Name, newName); err != nil {
		fmt.Fprintf(os.Stderr, "clotilde: failed to rename session: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "Session renamed to '%s' (detected /rename)\n", newName)
	sess.Name = newName // caller should pass updated sess.Name to detectBranchRenames
}

// detectBranchRenames scans branch sessions of parentSess for user-provided names.
// /branch emits a custom-title event ("name (Branch)") in the branch transcript;
// this strips the suffix, sanitizes to slug form, and renames auto-generated names.
func detectBranchRenames(clotildeRoot string, parentSess *session.Session) {
	store := session.NewFileStore(clotildeRoot)

	sessions, err := store.List()
	if err != nil {
		return
	}

	for _, s := range sessions {
		if s.Metadata.ParentSession != parentSess.Name || !s.Metadata.IsForkedSession {
			continue
		}
		// Only touch names we auto-generated (e.g. "parent-branch-1")
		if !isBranchAutoName(s.Name, parentSess.Name) {
			continue
		}
		if s.Metadata.TranscriptPath == "" {
			continue
		}

		customTitle, found := lastCustomTitle(s.Metadata.TranscriptPath)
		if !found {
			continue
		}

		newName := sanitizeBranchName(customTitle)
		if newName == "" || newName == s.Name {
			continue
		}

		if err := session.ValidateName(newName); err != nil {
			continue
		}
		if store.Exists(newName) {
			continue
		}

		if err := store.Rename(s.Name, newName); err != nil {
			fmt.Fprintf(os.Stderr, "clotilde: failed to rename branch session: %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "Branch renamed to '%s' (from /branch name)\n", newName)
	}
}

// sanitizeBranchName converts a Claude branch title to a valid clotilde session name.
// Strips the " (Branch)" suffix that Claude appends, lowercases, replaces non-slug
// characters with dashes, and collapses consecutive dashes.
func sanitizeBranchName(title string) string {
	name := strings.TrimSuffix(strings.TrimSpace(title), " (Branch)")
	name = strings.ToLower(strings.TrimSpace(name))

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name = strings.Trim(b.String(), "-")
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return name
}

// isBranchAutoName reports whether name matches the auto-generated pattern "<parent>-branch-N".
func isBranchAutoName(name, parentName string) bool {
	prefix := parentName + "-branch-"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(name, prefix)
	if len(suffix) == 0 {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// lastCustomTitle scans a JSONL transcript and returns the last custom-title value found.
// Uses bufio.Reader to handle long lines without hitting scanner limits.
func lastCustomTitle(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()

	var last string
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			var ev struct {
				Type        string `json:"type"`
				CustomTitle string `json:"customTitle"`
			}
			if json.Unmarshal([]byte(line), &ev) == nil && ev.Type == "custom-title" && ev.CustomTitle != "" {
				last = ev.CustomTitle
			}
		}
		if err != nil {
			break
		}
	}

	if last == "" {
		return "", false
	}
	return last, true
}
