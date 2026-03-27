package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
var SessionUsedFunc = DefaultSessionUsed

// InvokeOptions contains options for invoking claude CLI.
type InvokeOptions struct {
	SessionID        string
	Resume           bool
	SettingsFile     string
	SystemPromptFile string
	SystemPromptMode string // "append" (default) or "replace"
	AdditionalArgs   []string
	Env              map[string]string
}

// appendCommonArgs adds settings and system prompt flags to the arg list.
func appendCommonArgs(args []string, settingsFile, systemPromptFile, systemPromptMode string) []string {
	if settingsFile != "" && util.FileExists(settingsFile) {
		args = append(args, "--settings", settingsFile)
	}

	if systemPromptFile != "" && util.FileExists(systemPromptFile) {
		if systemPromptMode == "replace" {
			args = append(args, "--system-prompt-file", systemPromptFile)
		} else {
			args = append(args, "--append-system-prompt-file", systemPromptFile)
		}
	}

	return args
}

// Start invokes claude CLI to start a new session.
func Start(clotildeRoot string, sess *session.Session, settingsFile, systemPromptFile string, additionalArgs []string) error {
	args := []string{"--session-id", sess.Metadata.SessionID, "-n", sess.Name}
	args = appendCommonArgs(args, settingsFile, systemPromptFile, sess.Metadata.GetSystemPromptMode())
	args = append(args, additionalArgs...)

	env := map[string]string{
		"CLOTILDE_SESSION_NAME": sess.Name,
	}

	if sess.Metadata.IsIncognito {
		return invokeWithCleanup(clotildeRoot, sess, args, env)
	}

	err := invokeInteractive(args, env)
	cleanupEmptySession(clotildeRoot, sess)
	return err
}

// Resume invokes claude CLI to resume an existing session.
func Resume(clotildeRoot string, sess *session.Session, settingsFile, systemPromptFile string, additionalArgs []string) error {
	args := []string{"--resume", sess.Metadata.SessionID, "-n", sess.Name}
	args = appendCommonArgs(args, settingsFile, systemPromptFile, sess.Metadata.GetSystemPromptMode())
	args = append(args, additionalArgs...)

	env := map[string]string{
		"CLOTILDE_SESSION_NAME": sess.Name,
	}

	if sess.Metadata.IsIncognito {
		return invokeWithCleanup(clotildeRoot, sess, args, env)
	}

	return invokeInteractive(args, env)
}

// Fork invokes claude CLI to fork an existing session.
// The parent session will be resumed with --fork-session flag.
// For ephemeral forks, cleanup will happen when Claude exits.
func Fork(clotildeRoot string, parentSess *session.Session, forkName string, settingsFile, systemPromptFile string, additionalArgs []string, forkSession *session.Session) error {
	args := []string{"--resume", parentSess.Metadata.SessionID, "--fork-session", "--session-id", forkSession.Metadata.SessionID, "-n", forkName}
	args = appendCommonArgs(args, settingsFile, systemPromptFile, forkSession.Metadata.GetSystemPromptMode())
	args = append(args, additionalArgs...)

	env := map[string]string{
		"CLOTILDE_SESSION_NAME": forkName,
	}

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
// for a transcript file. Sessions with no ID are considered unused.
func DefaultSessionUsed(clotildeRoot string, sess *session.Session) bool {
	sessionID := sess.Metadata.SessionID
	if sessionID == "" {
		return false
	}

	// Prefer the transcript path saved by the hook (accurate even with symlinks),
	// fall back to computing it from the clotilde root.
	if sess.Metadata.TranscriptPath != "" {
		return util.FileExists(sess.Metadata.TranscriptPath)
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

// InvokeStreaming runs claude in non-interactive mode, streaming output to a callback.
// Each line of stdout is passed to onLine. Returns when the process exits.
// Canceling ctx kills the claude process.
func InvokeStreaming(ctx context.Context, opts InvokeOptions, prompt string, onLine func(line string)) error {
	var args []string

	if opts.Resume {
		args = append(args, "--resume", opts.SessionID)
	} else {
		args = append(args, "--session-id", opts.SessionID)
	}

	args = appendCommonArgs(args, opts.SettingsFile, opts.SystemPromptFile, opts.SystemPromptMode)
	args = append(args, "-p", prompt, "--output-format", "stream-json", "--verbose")
	args = append(args, opts.AdditionalArgs...)

	claudeBin := ClaudeBinaryPathFunc()

	if VerboseFunc() {
		displayCommand(claudeBin, args, opts.Env)
	}

	cmd := exec.CommandContext(ctx, claudeBin, args...)
	stderrTail := &tailBuffer{maxSize: 4096}
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrTail)

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range opts.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	var lastResult string
	reader := bufio.NewReader(stdout)
	var readErr error
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			// Capture any result event (error or success) for error reporting
			var ev struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal([]byte(line), &ev) == nil && ev.Type == "result" && ev.Result != "" {
				lastResult = ev.Result
			}
			onLine(line)
		}
		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
	}

	if readErr != nil {
		return fmt.Errorf("error reading stdout: %w", readErr)
	}

	if err := cmd.Wait(); err != nil {
		if lastResult != "" {
			return fmt.Errorf("claude error: %s", lastResult)
		}
		if stderr := strings.TrimSpace(stderrTail.String()); stderr != "" {
			return fmt.Errorf("claude exited with error: %w\n%s", err, stderr)
		}
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}

// tailBuffer is a bounded writer that keeps only the last maxSize bytes.
// Used to capture stderr tail for error messages without unbounded memory growth.
type tailBuffer struct {
	maxSize int
	buf     []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.maxSize {
		t.buf = t.buf[len(t.buf)-t.maxSize:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	return string(t.buf)
}

