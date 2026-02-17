package claude

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

// VerboseFunc is a function that returns whether verbose mode is enabled.
// This is set by the cmd package.
var VerboseFunc func() bool = func() bool { return false }

// SessionUsedFunc checks if a Claude Code session was actually used (has a transcript).
// Can be overridden in tests where the fake claude binary doesn't create transcripts.
var SessionUsedFunc = defaultSessionUsed

// InvokeOptions contains options for invoking claude CLI.
type InvokeOptions struct {
	SessionID        string
	Resume           bool
	ForkSession      bool
	SettingsFile     string
	SystemPromptFile string
	SystemPromptMode string // "append" (default) or "replace"
	AdditionalArgs   []string
	Env              map[string]string
}

// Start invokes claude CLI to start a new session.
func Start(clotildeRoot string, sess *session.Session, settingsFile, systemPromptFile string, additionalArgs []string) error {
	args := []string{
		"--session-id", sess.Metadata.SessionID,
	}

	// Add settings file if it exists
	if settingsFile != "" && util.FileExists(settingsFile) {
		args = append(args, "--settings", settingsFile)
	}

	// Add system prompt file if it exists (use correct flag based on mode)
	if systemPromptFile != "" && util.FileExists(systemPromptFile) {
		mode := sess.Metadata.GetSystemPromptMode()
		if mode == "replace" {
			args = append(args, "--system-prompt-file", systemPromptFile)
		} else {
			// Default to append if mode not explicitly set
			args = append(args, "--append-system-prompt-file", systemPromptFile)
		}
	}

	// Add additional args (pass-through flags)
	args = append(args, additionalArgs...)

	// Set environment variable for the hook
	env := map[string]string{
		"CLOTILDE_SESSION_NAME": sess.Name,
	}

	// Use cleanup wrapper for incognito sessions
	if sess.Metadata.IsIncognito {
		return invokeWithCleanup(clotildeRoot, sess, args, env)
	}

	err := invokeInteractive(args, env)
	cleanupEmptySession(clotildeRoot, sess)
	return err
}

// Resume invokes claude CLI to resume an existing session.
func Resume(clotildeRoot string, sess *session.Session, settingsFile, systemPromptFile string, additionalArgs []string) error {
	args := []string{
		"--resume", sess.Metadata.SessionID,
	}

	// Add settings file if it exists
	if settingsFile != "" && util.FileExists(settingsFile) {
		args = append(args, "--settings", settingsFile)
	}

	// Add system prompt file if it exists (use correct flag based on mode)
	if systemPromptFile != "" && util.FileExists(systemPromptFile) {
		mode := sess.Metadata.GetSystemPromptMode()
		if mode == "replace" {
			args = append(args, "--system-prompt-file", systemPromptFile)
		} else {
			// Default to append if mode not explicitly set
			args = append(args, "--append-system-prompt-file", systemPromptFile)
		}
	}

	// Add additional args (pass-through flags)
	args = append(args, additionalArgs...)

	// Set environment variable for the hook
	env := map[string]string{
		"CLOTILDE_SESSION_NAME": sess.Name,
	}

	// Use cleanup wrapper for incognito sessions
	if sess.Metadata.IsIncognito {
		return invokeWithCleanup(clotildeRoot, sess, args, env)
	}

	return invokeInteractive(args, env)
}

// Fork invokes claude CLI to fork an existing session.
// The parent session will be resumed with --fork-session flag.
// Environment variables should include CLOTILDE_FORK_NAME and CLOTILDE_PARENT_SESSION.
// For ephemeral forks, cleanup will happen when Claude exits.
func Fork(clotildeRoot string, parentSess *session.Session, forkName string, settingsFile, systemPromptFile string, additionalArgs []string, forkSession *session.Session) error {
	args := []string{
		"--resume", parentSess.Metadata.SessionID,
		"--fork-session",
	}

	// Add settings file if it exists
	if settingsFile != "" && util.FileExists(settingsFile) {
		args = append(args, "--settings", settingsFile)
	}

	// Add system prompt file if it exists (use correct flag based on fork's mode)
	if systemPromptFile != "" && util.FileExists(systemPromptFile) {
		if forkSession.Metadata.GetSystemPromptMode() == "replace" {
			args = append(args, "--system-prompt-file", systemPromptFile)
		} else {
			args = append(args, "--append-system-prompt-file", systemPromptFile)
		}
	}

	// Add additional args (pass-through flags)
	args = append(args, additionalArgs...)

	// Set environment variables for the hook
	env := map[string]string{
		"CLOTILDE_SESSION_NAME":   forkName,
		"CLOTILDE_FORK_NAME":      forkName,
		"CLOTILDE_PARENT_SESSION": parentSess.Name,
	}

	// For incognito forks, use cleanup wrapper
	if forkSession.Metadata.IsIncognito {
		return invokeWithCleanup(clotildeRoot, forkSession, args, env)
	}

	err := invokeInteractive(args, env)
	cleanupEmptySession(clotildeRoot, forkSession)
	return err
}

// ClaudeBinaryPathFunc is a function that returns the path to the claude binary.
// This is set by the cmd package to allow overriding for tests.
var ClaudeBinaryPathFunc func() string = func() string { return "claude" }

// displayCommand prints the command being executed (always shown) and verbose debug info (if verbose mode).
func displayCommand(claudeBin string, args []string, env map[string]string) {
	// Always display the command being executed
	cmdStr := claudeBin + " " + strings.Join(args, " ")
	fmt.Fprintf(os.Stderr, "→ %s\n", cmdStr)

	// Show additional debug info in verbose mode
	if VerboseFunc() {
		if len(env) > 0 {
			fmt.Fprintln(os.Stderr, "[DEBUG] Environment variables:")
			for k, v := range env {
				fmt.Fprintf(os.Stderr, "  %s=%s\n", k, v)
			}
		}
	}
}

// invokeInteractive executes the claude CLI command interactively.
// Stdin, stdout, and stderr are connected to the current process.
func invokeInteractive(args []string, env map[string]string) error {
	claudeBin := ClaudeBinaryPathFunc()

	// Display the command being executed
	displayCommand(claudeBin, args, env)

	cmd := exec.Command(claudeBin, args...)

	// Set up stdio
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return cmd.Run()
}

// invokeWithCleanup runs claude and cleans up incognito session on exit.
// Uses defer to ensure cleanup runs even on panic or interrupt (Ctrl+C).
func invokeWithCleanup(clotildeRoot string, sess *session.Session, args []string, env map[string]string) error {
	// Setup cleanup to run after claude exits (even on panic/Ctrl+C)
	defer func() {
		deleted, err := cleanupIncognitoSession(clotildeRoot, sess)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Warning(fmt.Sprintf("Failed to cleanup incognito session: %v", err)))
		} else {
			fmt.Println(ui.Info(fmt.Sprintf("👻 Deleted incognito session '%s'", sess.Name)))

			// Show detailed info in verbose mode
			if VerboseFunc() {
				transcriptCount := len(deleted.Transcript)
				agentLogCount := len(deleted.AgentLogs)
				fmt.Printf("  Session folder, %d transcript(s), %d agent log(s)\n", transcriptCount, agentLogCount)

				if transcriptCount > 0 {
					fmt.Println("\n  Deleted transcripts:")
					for _, path := range deleted.Transcript {
						fmt.Printf("    %s\n", path)
					}
				}
				if agentLogCount > 0 {
					fmt.Println("\n  Deleted agent logs:")
					for _, path := range deleted.AgentLogs {
						fmt.Printf("    %s\n", path)
					}
				}
			}
		}
	}()

	// Run claude (blocks until exit)
	return invokeInteractive(args, env)
}

// cleanupIncognitoSession deletes session folder and Claude data.
// Returns DeletedFiles with info about what was deleted.
func cleanupIncognitoSession(clotildeRoot string, sess *session.Session) (*DeletedFiles, error) {
	deleted := &DeletedFiles{
		Transcript: []string{},
		AgentLogs:  []string{},
	}

	// Delete Claude data (transcript + agent logs)
	claudeDeleted, err := DeleteSessionData(clotildeRoot, sess.Metadata.SessionID, sess.Metadata.TranscriptPath)
	if err != nil {
		// Log but continue - session folder cleanup is more important
		fmt.Fprintf(os.Stderr, "Warning: failed to delete Claude data: %v\n", err)
	} else {
		deleted.Transcript = append(deleted.Transcript, claudeDeleted.Transcript...)
		deleted.AgentLogs = append(deleted.AgentLogs, claudeDeleted.AgentLogs...)
	}

	// Delete session folder
	store := session.NewFileStore(clotildeRoot)
	if err := store.Delete(sess.Name); err != nil {
		return deleted, err
	}

	return deleted, nil
}

// defaultSessionUsed checks if a Claude Code session was actually used by looking
// for a transcript file. Sessions with no ID (e.g., forks where the hook didn't run)
// are considered unused.
func defaultSessionUsed(clotildeRoot string, sess *session.Session) bool {
	sessionID := sess.Metadata.SessionID
	if sessionID == "" {
		return false
	}
	homeDir, err := util.HomeDir()
	if err != nil {
		return true // assume used if we can't check
	}
	transcriptPath := TranscriptPath(homeDir, clotildeRoot, sessionID)
	return util.FileExists(transcriptPath)
}

// cleanupEmptySession removes a session if Claude Code never created a transcript.
// This handles the case where the user starts a session but exits without sending
// any messages, leaving a ghost session in clotilde's store.
func cleanupEmptySession(clotildeRoot string, sess *session.Session) {
	// Reload session from disk (hook may have updated metadata)
	store := session.NewFileStore(clotildeRoot)
	current, err := store.Get(sess.Name)
	if err != nil {
		// Session doesn't exist (already cleaned up), nothing to do
		return
	}

	if !SessionUsedFunc(clotildeRoot, current) {
		if err := store.Delete(current.Name); err != nil {
			fmt.Fprintln(os.Stderr, ui.Warning(fmt.Sprintf("Failed to cleanup empty session: %v", err)))
			return
		}
		fmt.Fprintln(os.Stderr, ui.Info(fmt.Sprintf("Removed empty session '%s' (no messages were sent)", current.Name)))
	}
}

// Invoke executes claude CLI with custom options (for advanced use cases).
func Invoke(opts InvokeOptions) error {
	var args []string

	if opts.Resume {
		args = append(args, "--resume", opts.SessionID)
		if opts.ForkSession {
			args = append(args, "--fork-session")
		}
	} else {
		args = append(args, "--session-id", opts.SessionID)
	}

	if opts.SettingsFile != "" && util.FileExists(opts.SettingsFile) {
		args = append(args, "--settings", opts.SettingsFile)
	}

	if opts.SystemPromptFile != "" && util.FileExists(opts.SystemPromptFile) {
		// Use correct flag based on mode (default to append)
		if opts.SystemPromptMode == "replace" {
			args = append(args, "--system-prompt-file", opts.SystemPromptFile)
		} else {
			args = append(args, "--append-system-prompt-file", opts.SystemPromptFile)
		}
	}

	args = append(args, opts.AdditionalArgs...)

	return invokeInteractive(args, opts.Env)
}
