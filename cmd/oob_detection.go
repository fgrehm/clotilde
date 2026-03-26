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
