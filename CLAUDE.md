# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Purpose

Clotilde is a Go CLI wrapper around Claude Code that adds named session management. It wraps Claude Code session UUIDs with human-friendly names, enabling easy switching between multiple parallel conversations in the same project.

Key capabilities:
- Named sessions (vs UUIDs)
- Session forking (branch conversations)
- Incognito sessions (auto-delete on exit)
- System prompt customization per session
- Full cleanup (sessions + Claude Code transcripts/logs)

## Architecture

### Core Concept

Clotilde is a **thin, non-invasive wrapper**. It:
- Generates UUIDs and stores name→UUID mappings in `.claude/clotilde/sessions/<name>/metadata.json`
- Invokes `claude` CLI with the mapped UUID
- Never modifies Claude Code itself

### Package Structure

```
cmd/                    # Cobra command implementations
  setup.go              # One-time global hook registration
  init.go               # Initialize clotilde (deprecated, use setup)
  start.go              # Start new session
  incognito.go          # Start incognito session (auto-deletes on exit)
  resume.go             # Resume existing session
  list.go               # List all sessions
  inspect.go            # Show detailed session info
  fork.go               # Fork session
  delete.go             # Delete session and Claude data
  hook.go               # Hidden hook parent command
  hook_sessionstart.go  # Unified SessionStart hook handler (startup/resume/compact/clear)
  tour.go               # Tour subcommands (list, serve, generate)
internal/
  session/              # Session data structures, storage (FileStore), validation
  config/               # Config management, path resolution
  claude/               # Claude CLI invocation, path conversion, hook generation, cleanup
  util/                 # UUID generation, filesystem helpers
  testutil/             # Test utilities (fake claude binary)
  tour/                 # Tour file loading, generation, validation, prompt construction
  server/               # HTTP server, WebSocket chat, static assets, REST API
main.go                 # Entry point
```

### Session Structure

Each session is a folder in `.claude/clotilde/sessions/<name>/`:

```
.claude/clotilde/
  config.json             # Project config (profiles - optional, created manually)
  sessions/
    my-session/
      metadata.json       # Session metadata (name, sessionId, timestamps, parent info)
      settings.json       # Claude Code settings (model, permissions - optional)
      system-prompt.md    # System prompt content (optional)
```

**Metadata format** (`metadata.json`):
```json
{
  "name": "my-feature",
  "sessionId": "uuid-for-claude-code",
  "transcriptPath": "/home/user/.claude/projects/.../uuid.jsonl",
  "created": "2025-11-23T10:30:00Z",
  "lastAccessed": "2025-11-23T18:42:00Z",
  "parentSession": "original-session",
  "isForkedSession": true,
  "isIncognito": false,
  "previousSessionIds": ["old-uuid-1", "old-uuid-2"],
  "context": "working on ticket GH-123"
}
```

**`previousSessionIds`**: Array of UUIDs from `/clear` operations. When Claude Code clears a session, it creates a new UUID. Clotilde tracks the old UUIDs here for complete cleanup on deletion. Note: `/compact` does NOT currently create a new UUID (only `/clear` does), but we handle it defensively in the code.

**`isIncognito`**: Boolean flag. If true, session auto-deletes on exit (via defer-based cleanup in `invoke.go`). Incognito sessions are useful for quick queries, experiments, or sensitive work. Cleanup runs on normal exit and Ctrl+C, but not on SIGKILL or crashes.

**`context`**: Optional free-text field set via `--context` flag on `start`, `incognito`, `fork`, and `resume` commands. Injected into Claude via the SessionStart hook alongside the session name. Forked sessions inherit context from the parent unless overridden. Context can be updated on resume (e.g. `clotilde resume my-session --context "now on GH-456"`).

**Project config format** (`.claude/clotilde/config.json`):
```json
{
  "profiles": {
    "quick": {
      "model": "haiku",
      "permissionMode": "bypassPermissions"
    },
    "strict": {
      "permissions": {
        "deny": ["Bash", "Write"],
        "defaultMode": "ask"
      }
    },
    "research": {
      "model": "sonnet",
      "outputStyle": "Explanatory"
    }
  }
}
```

**Global config format** (`~/.config/clotilde/config.json`):

Same structure as the project config. Respects `$XDG_CONFIG_HOME` if set, otherwise defaults to `~/.config/clotilde/config.json`. Profiles defined here are available in all projects.

**Config purpose**: Define named session presets (profiles) for common configurations. Use `clotilde start <name> --profile <profile>` to apply a profile.

**Profile fields**:
- `model` - Claude model (haiku, sonnet, opus)
- `permissionMode` - Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)
- `permissions` - Granular permissions: allow/deny/ask lists, additionalDirectories, defaultMode, disableBypassPermissionsMode
- `outputStyle` - Output style (built-in or custom name)

**Precedence**: Global profile → project profile → CLI flags (each layer overrides the previous). For example, if both global and project configs define a `"quick"` profile, the project version wins. CLI flags always override profile values.

**Settings format** (`settings.json`):
```json
{
  "model": "sonnet",
  "permissions": {
    "allow": ["Bash", "Read"],
    "deny": ["Write"],
    "ask": [],
    "additionalDirectories": [],
    "defaultMode": "ask",
    "disableBypassPermissionsMode": "false"
  }
}
```

**Settings scope**: Only session-specific settings (model, permissions). Not global stuff like hooks, MCP servers, status line. Settings file is ALWAYS created (empty object if no model/permissions specified).

**Context loading**: Context is injected at session start via SessionStart hooks:
- **Session name**: Always output if available
- **Session context**: From metadata `context` field (set via `--context` flag)

### Claude Code Integration Patterns

**Starting a session:**
```bash
claude --session-id <uuid> \
  --settings .claude/clotilde/sessions/<name>/settings.json \
  --append-system-prompt-file .claude/clotilde/sessions/<name>/system-prompt.md
```

**Resuming a session:**
```bash
claude --resume <uuid> \
  --settings .claude/clotilde/sessions/<name>/settings.json \
  --append-system-prompt-file .claude/clotilde/sessions/<name>/system-prompt.md
```

**Forking a session:**
```bash
claude --resume <parent-uuid> --fork-session \
  --session-id <fork-uuid> -n <fork-name> \
  [--settings ...] [--append-system-prompt-file ...]
```

Note: `--settings` and `--append-system-prompt-file` are only added if the files exist. `--session-id` pre-assigns the fork's UUID (avoids hook-based UUID registration). `-n` sets the display name shown in Claude's native session picker.

### Session Hooks

**Unified SessionStart hook** (`clotilde hook sessionstart`) handles all session lifecycle events internally based on the `source` field in JSON input:

**Hook registration** (in `~/.claude/settings.json`, installed by `clotilde setup`):
```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [{"type": "command", "command": "clotilde hook sessionstart"}]
      }
    ]
  }
}
```

No matcher field - the single hook handles all sources (startup, resume, compact, clear) internally.

**Source-based dispatch:**
- **`startup`**: New sessions - outputs session name and context, saves transcript path
- **`resume`**: Resuming, `clotilde fork`, or in-session `/branch` - outputs context; detects UUID mismatch to auto-register `/branch` as new clotilde session (see below)
- **`compact`**: Session compaction - defensive handler (Claude Code doesn't currently create new UUID for `/compact`, but we handle it anyway in case behavior changes)
- **`clear`**: Session clear - updates metadata with new UUID, preserves old UUID in `previousSessionIds` array

**`clotilde fork` registration:**
1. `clotilde fork` pre-assigns a UUID via `util.GenerateUUID()` before creating the session
2. Sets env var: `CLOTILDE_SESSION_NAME` (for context output in hook)
3. Invokes `claude --resume <parent> --fork-session --session-id <forkUUID> -n <forkName>`
4. Claude triggers SessionStart with `source: "resume"` → hook outputs context for the new session
5. Fork UUID is guaranteed to match because it was pre-assigned (no hook-based UUID registration needed)

**In-session `/branch` detection (hook-time):**
1. User runs `/branch [name]` inside Claude Code
2. Claude creates a new UUID and triggers SessionStart with `source: "resume"`
3. Hook sees `CLOTILDE_SESSION_NAME` set but the incoming UUID doesn't match the registered session
4. Hook reads the first line of the new transcript to verify `forkedFrom.sessionId` matches parent
5. Generates branch name as `<parent>-branch-N` (incrementing), creates new clotilde session
6. Outputs "Clotilde: registered branch as '<name>'" to stderr
7. Writes fork name to `CLAUDE_ENV_FILE` for correct `/clear` tracking inside the branch

**Post-exit `/branch` rename detection:**
After `invokeInteractive` returns in `claude.Resume()`/`claude.Start()`, `detectBranchRenames()` in `cmd/slash_detect.go`:
1. Lists all sessions with `IsForkedSession=true` and `ParentSession` matching the current session
2. For each session whose name matches the auto-generated `<parent>-branch-N` pattern
3. Reads the branch transcript for the last `custom-title` entry (written by Claude when user provides a name)
4. Strips ` (Branch)` suffix Claude appends, sanitizes to slug format
5. Renames the session if the result is valid and not already taken

**Clear handling:**
1. User runs `/clear` in Claude Code
2. Claude creates new UUID and triggers SessionStart with `source: "clear"`
3. Hook resolves session name using three-level fallback:
   - Priority 1: `CLOTILDE_SESSION_NAME` env var (from `clotilde resume`)
   - Priority 2: Read from `CLAUDE_ENV_FILE` (persisted by previous hook)
   - Priority 3: Reverse UUID lookup in sessions (searches current and previous IDs)
4. Hook calls `session.AddPreviousSessionID()` to update metadata:
   - Appends current UUID to `previousSessionIds` array (idempotent)
   - Updates `sessionId` to new UUID
5. Session name persists across multiple `/clear` operations

**Note on `/compact`:** Currently, Claude Code does NOT create a new session UUID when `/compact` is run (only `/clear` does). However, the hook defensively handles `source: "compact"` identically to `source: "clear"` in case Claude Code's behavior changes in the future.

**Post-exit `/rename` detection:**
After `invokeInteractive` returns in `claude.Resume()`/`claude.Start()`, `detectRename()` in `cmd/slash_detect.go`:
1. Reads the session's transcript and scans for the last `{"type":"custom-title","customTitle":"..."}` entry
2. If found and differs from the current session name, validates the new name (must pass `session.ValidateName` slug format)
3. Calls `store.Rename(oldName, newName)` which renames the session directory on disk
4. Logs a warning to stderr if the name fails validation (Claude allows arbitrary names; clotilde requires slug format)
5. Updates `sess.Name` in-process so `detectBranchRenames` uses the updated name

**Context loading:**
- Hook outputs context to stdout which gets automatically injected by Claude Code
- Session name is always output if available (e.g. "Session name: my-feature")
- Session context from metadata is output if set (e.g. "Context: working on GH-123")
- Hooks use os.Stdin piping to read JSON input from Claude Code

### Claude Code Path Conversion

Claude Code stores project data in `~/.claude/projects/` with paths like:
```
/home/user/project/foo.bar → ~/.claude/projects/-home-user-project-foo-bar/
```

Conversion rule: Replace `/` and `.` with `-`

Implementation in `internal/claude/paths.go`:
- `ProjectDir(clotildeRoot)` - Converts `.claude/clotilde` parent to Claude's project dir format
- Used for deleting transcripts/agent logs

### Delete Behavior

When deleting a session, remove:
- Session folder: `.claude/clotilde/sessions/<name>/`
- Claude transcript (current): `~/.claude/projects/<project-dir>/<uuid>.jsonl`
- Claude transcripts (previous): For each UUID in `previousSessionIds` array
- Agent logs: `~/.claude/projects/<project-dir>/agent-*.jsonl` (grep for all sessionIds)

This ensures complete cleanup even after multiple `/clear` operations (and `/compact`, if Claude Code's behavior changes to create new UUIDs for compaction).

## Interactive Codebase Tours (Experimental)

Tours are browser-based interactive walkthroughs of a codebase, integrating a code viewer with a Claude Code chat sidebar. Tours enable developers to explore unfamiliar code while asking questions to Claude with full context awareness.

### Tour Architecture

**Three main subsystems:**

1. **Tour file format** (`internal/tour/`)
   - CodeTour JSON format: `{title, steps: [{file, line, description}]}`
   - `LoadFile()` / `LoadFromDir()` — Parse and validate tour files
   - `ValidateTourJSON()` — Validate generated tours against actual repo
   - `BuildGenerationPrompt()` — Construct prompt telling Claude to crawl the repo autonomously

2. **Non-interactive Claude invocation** (`internal/claude/invoke.go`)
   - `InvokeStreaming(opts, prompt, onLine)` — Spawn claude CLI, capture streaming JSON
   - Used by tour generation (`tour generate`) and chat sidebar
   - Handles `--session-id` (first call) and `--resume` (continuation)
   - Captures stream-json output line-by-line, no user interaction

3. **HTTP server + browser UI** (`internal/server/`)
   - REST API: `/api/tours`, `/api/files/{path}`, `/api/tree`, `/api/session`
   - WebSocket: `/ws/chat` — bidirectional chat with streaming response
   - Static assets: embedded HTML/CSS/JS via `embed.FS`
   - Persistent session management: tour chat uses `tour-<repo-name>` session with full system prompt replacement

### Tour Generation Flow

1. User runs `clotilde tour generate --focus "auth"`
2. `BuildGenerationPrompt()` builds a prompt telling Claude to crawl the repo with its own tools (max 20 files, 8-15 steps)
3. `InvokeStreaming()` runs claude with `--permission-mode bypassPermissions`, capturing JSON output; tool calls are streamed as progress to stderr
4. `ValidateTourJSON()` checks: valid JSON, files exist, lines in range
5. Writes `.tours/<name>.tour` or `.tours/<name>.tour.invalid` on failure
6. Prints summary to stderr

### Tour Serving Flow

1. `clotilde tour serve` starts HTTP server on localhost
2. Creates/loads `tour-<repo-name>` Clotilde session with:
   - System prompt replacement (tour guide role)
   - Output style "explanatory"
   - Model from `--model` flag (default: haiku)
3. Browser loads tour list, displays code viewer + tour nav + chat panel
4. Chat messages include context: tour name, step, file, line, description
5. Each message resumes the same session (full continuity)
6. Chat responses rendered as markdown with syntax highlighting

### Key Design Decisions

- **System prompt replacement (not append)** — Full control over tour guide role
- **Non-interactive invocation** — `InvokeStreaming()` captures output cleanly, no TTY needed
- **Persistent sessions** — Tour chat uses Clotilde infrastructure for full history
- **Context injection in prompt** — No special backend logic; Claude sees tour context inline
- **Streaming JSON capture** — Works line-by-line, no buffering, low memory

### Test Infrastructure

- Fake claude binary (`internal/testutil/`) for integration tests
- `server.InvokeStreamingFunc` injectable for testing chat without real claude
- Mock tour files + source code for server tests
- WebSocket tests via `github.com/coder/websocket`

## Build & Test

**Setup git hooks** (recommended first step):
```bash
make setup-hooks   # Enables pre-commit checks for formatting and linting
```

**Common commands:**
```bash
make build         # Build to dist/clotilde
make test          # Run all tests
make test-watch    # Run tests in watch mode
make coverage      # Generate coverage report
make fmt           # Format code
make lint          # Run linter
make install       # Install to ~/.local/bin
make clean         # Remove artifacts
```

**Test Organization:**
- 7 test suites: `internal/util/`, `internal/config/`, `internal/session/`, `internal/claude/`, `internal/outputstyle/`, `internal/ui/`, `cmd/`
- Unit tests for core functionality
- Integration tests using fake claude binary (internal/testutil)
- os.Pipe() for testing hook stdin/stdout communication
- Isolated test environments with temp directories

**Testing Philosophy:**
- **Write tests alongside code changes** - new features and bug fixes should include test coverage
- Test both success and error cases
- Keep tests focused and independent
- Use descriptive test names

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for current status and future ideas.

## Documentation

### Core Concepts

- **[Claude Settings Behavior](docs/claude-settings-behavior.md)** - Detailed analysis of how Claude Code's `--settings` flag, permission system, and multi-layer settings work. Critical for understanding Clotilde's design decisions around session isolation and permission handling.

## Key Constraints

- **Minimal wrapper**: Don't reinvent Claude Code features, just wrap them
- **Non-invasive**: Never patch or modify Claude Code binaries
- **Stable format**: Session structure should remain consistent across versions
- **Single binary**: No runtime dependencies (Go only)
- **Settings scope**: `settings.json` should only contain session-specific settings (model, permissions), not global config (hooks, MCP, UI)
- **Native integration**: Use `--settings` flag to pass settings, let Claude Code handle merging with global/project configs
