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
  init.go               # Initialize clotilde structure and hooks
  start.go              # Start new session
  incognito.go          # Start incognito session (auto-deletes on exit)
  resume.go             # Resume existing session
  list.go               # List all sessions
  inspect.go            # Show detailed session info
  fork.go               # Fork session
  delete.go             # Delete session and Claude data
  hook.go               # Hidden hook parent command
  hook_sessionstart.go  # Unified SessionStart hook handler (startup/resume/compact/clear)
internal/
  session/              # Session data structures, storage (FileStore), validation
  config/               # Config management, path resolution
  claude/               # Claude CLI invocation, path conversion, hook generation, cleanup
  util/                 # UUID generation, filesystem helpers
  testutil/             # Test utilities (fake claude binary)
main.go                 # Entry point
```

### Session Structure

Each session is a folder in `.claude/clotilde/sessions/<name>/`:

```
.claude/clotilde/
  config.json             # Global config (profiles, etc - optional)
  context.md              # Global context for all sessions (optional)
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
  "previousSessionIds": ["old-uuid-1", "old-uuid-2"]
}
```

**`previousSessionIds`**: Array of UUIDs from `/clear` operations. When Claude Code clears a session, it creates a new UUID. Clotilde tracks the old UUIDs here for complete cleanup on deletion. Note: `/compact` does NOT currently create a new UUID (only `/clear` does), but we handle it defensively in the code.

**`isIncognito`**: Boolean flag. If true, session auto-deletes on exit (via defer-based cleanup in `invoke.go`). Incognito sessions are useful for quick queries, experiments, or sensitive work. Cleanup runs on normal exit and Ctrl+C, but not on SIGKILL or crashes.

**Global config format** (`config.json`):
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

**Config purpose**: Define named session presets (profiles) for common configurations. Use `clotilde start <name> --profile <profile>` to apply a profile.

**Profile fields**:
- `model` - Claude model (haiku, sonnet, opus)
- `permissionMode` - Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)
- `permissions` - Granular permissions: allow/deny/ask lists, additionalDirectories, defaultMode, disableBypassPermissionsMode
- `outputStyle` - Output style (built-in or custom name)

**Precedence**: Profile values → CLI flags (CLI flags always override profile values). For example, `--profile quick --model opus` uses the quick profile but overrides its model with opus.

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

**Context loading**: Global context loaded at session start via SessionStart hooks:
- **Global** (`.claude/clotilde/context.md`): What you're working on (ticket info, task goal, relevant specs)
- Hook outputs context with a header indicating the source file so Claude knows where to make updates if needed

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
  [--settings ...] [--append-system-prompt-file ...]
```

Note: `--settings` and `--append-system-prompt-file` are only added if the files exist.

### Session Hooks

**Unified SessionStart hook** (`clotilde hook sessionstart`) handles all session lifecycle events internally based on the `source` field in JSON input:

**Hook registration** (in `.claude/settings.json`):
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
- **`startup`**: New sessions - outputs global context, saves transcript path
- **`resume`**: Resuming/forking - registers fork UUID if `CLOTILDE_FORK_NAME` env var set, outputs context
- **`compact`**: Session compaction - defensive handler (Claude Code doesn't currently create new UUID for `/compact`, but we handle it anyway in case behavior changes)
- **`clear`**: Session clear - updates metadata with new UUID, preserves old UUID in `previousSessionIds` array

**Fork registration:**
1. `clotilde fork` creates fork folder with empty `sessionId` in metadata.json
2. Sets env vars: `CLOTILDE_FORK_NAME`, `CLOTILDE_PARENT_SESSION`, `CLOTILDE_SESSION_NAME`
3. Invokes `claude --resume <parent> --fork-session`
4. Claude triggers SessionStart with `source: "resume"` → hook detects `CLOTILDE_FORK_NAME` and updates fork metadata with new UUID
5. Hook is idempotent (won't overwrite existing UUIDs)

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

**Context loading:**
- Hook outputs context to stdout which gets automatically injected by Claude Code
- Global context (`.claude/clotilde/context.md`): What you're working on (ticket/issue info, task goal)
- Hook outputs a header ("Clotilde session context source / --- Loaded from .claude/clotilde/context.md ---") so Claude knows where to make updates if needed
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
