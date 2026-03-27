# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`effortLevel` persisted in `settings.json`**: The `--effort` flag (and `--fast`, which implies `effort: low`) is now stored in the session's `settings.json` as `effortLevel`. This means the effort level is automatically applied on resume without needing to pass `--effort` again.

### Fixed

- **`clotilde fork` UUID mismatch**: Previously, forked sessions ended up with the parent's UUID because `SessionStart` fires with the parent's UUID when using `--fork-session`. Fixed by pre-assigning a UUID with `--session-id` before invocation (same pattern as `start`). Also passes `-n <name>` to `start`, `resume`, and `fork` so clotilde session names appear in Claude's native session picker.
- **Propagate errors in fork custom output style handling**: `json.MarshalIndent`, `os.WriteFile`, and `store.Update` errors during fork style inheritance were previously silently discarded. They now return errors so forks fail loudly on inconsistent state.

## [0.10.0] - 2026-03-24

### Added

- **`--effort` flag**: Pass reasoning effort level (`low`, `medium`, `high`, `max`) through to Claude CLI on `start`, `resume`, `fork`, and `incognito` commands. Conflicts with `--fast` (which sets effort to `low` automatically). Includes shell completion for valid values.
- **`--model` on resume**: Override the model when resuming a session (e.g. `clotilde resume my-session --model opus`). Previously `--model` was only available on `start` and `incognito`.

### Fixed

- **Session leakage into `$HOME`**: `ProjectRootFromPath` now stops walking up at `$HOME`, preventing `~/.claude/` (Claude Code's global config) from being treated as a project marker. Previously, projects without their own `.claude/` directory would create sessions under `~/.claude/clotilde/sessions/`.

## [0.9.0] - 2026-03-23

### Added

- **Interactive Codebase Tours (experimental)**: New `clotilde tour` subcommand for browser-based interactive codebase walkthroughs with integrated Claude chat.
  - `clotilde tour list` — List available tours in `.tours/` directory
  - `clotilde tour serve [--port] [--model]` — Start interactive tour web server with code viewer, tour navigation, and chat sidebar
  - `clotilde tour generate [--focus] [--model]` — Generate tours automatically by analyzing the codebase with Claude
- **Tour features**:
  - **Persistent chat sessions** — Tours create a persistent `tour-<repo-name>` Clotilde session, preserving chat history across browser refreshes
  - **System prompt replacement** — Tour guide role fully replaces Claude's default system prompt (not appended) for focused, tour-specific context
  - **URL persistence** — Current step is saved to URL query parameter (`?step=N`) for bookmarking and resumability
  - **Chat reset** — Button to clear chat history and start a fresh conversation
  - **Tour format** — Supports CodeTour JSON format (`.tours/<name>.tour`)
- **Streaming JSON output** — New `InvokeStreaming()` function in `internal/claude/` for non-interactive Claude invocations with streaming JSON output capture

## [0.8.1] - 2026-03-17

### Fixed

- **`--` passthrough with positional args**: Commands that support `-- <claude-flags>` (`start`, `resume`, `incognito`, `fork`) now correctly accept extra flags after `--`. Previously, Cobra's built-in arg validators counted args after `--` as positional args, causing an "accepts at most 1 arg(s), received 2" error (e.g. `clotilde start my-session -- LFG`).

## [0.8.0] - 2026-03-16

### Added

- **Git branch auto-naming**: Auto-naming commands (`start`, `incognito`, `fork`, and dashboard quick-actions) now use the current git branch name as the session name when not on `main` or `master` (e.g. branch `feature/gh-456` → session `feature-gh-456`). If the branch name is already taken, a numeric suffix is appended (`-2` through `-9`). Falls back to the existing `YYYY-MM-DD-adjective-noun` format on trunk branches, detached HEAD, or outside a git repo.
- **Full transcript history in `stats` and `export`**: Both commands now include all previous transcripts (from `/clear` operations tracked in `previousSessionIds`). `stats` sums turns across the full history and shows the earliest start time and latest activity. `export` produces HTML covering the complete conversation from the first message.
- **SessionEnd stats recording**: Opt-in SessionEnd hook that records session statistics (turns, tokens, models, tool usage) to daily JSONL files at `$XDG_DATA_HOME/clotilde/stats/`. Enable with `clotilde setup --stats`, disable with `--no-stats`. Includes crash recovery for sessions that exit without triggering SessionEnd.
- **`stats --all` flag**: Show aggregate stats across sessions active in the last 7 days (scoped to current project). Reads from JSONL stats files when available, falls back to transcript parsing.
- **`stats backfill` subcommand**: Generate JSONL stats records from existing session transcripts. Useful for populating stats after enabling tracking on an existing project.
- **Rich `stats` output**: Per-session stats now include token counts (input, output, cache read, cache write), models used, and tool usage breakdown (sorted by count, internal orchestration tools filtered out).

### Changed

- **`clotilde ls` model column**: Reads only the last 128KB of each transcript file instead of the full file, significantly reducing load time for projects with many or large sessions.
- **`clotilde ls` last-used column**: Now reads the timestamp of the last entry in the transcript instead of the file mtime, giving a more accurate and meaningful activity time. Also updated on every hook-driven session start, not just on explicit `clotilde resume`.

### Fixed

- **Third-party hook preservation**: `clotilde setup` now correctly preserves non-clotilde hooks (e.g. zellaude) when merging hook configuration, instead of stripping them.

## [0.7.0] - 2026-03-11

### Added

- **`setup` command**: `clotilde setup` registers SessionStart hooks in `~/.claude/settings.json` (global). Run once after installing. Supports `--local` flag for `~/.claude/settings.local.json`. Idempotent and merges with existing settings.
- **Lazy session directory creation**: `clotilde start` (and other session-creating commands) automatically create `.claude/clotilde/sessions/` on first use. No `init` required.
- **Double-hook execution guard**: Prevents duplicate context output when both global and per-project hooks exist (migration safety).
- **`export` command**: `clotilde export <name>` renders a session transcript into a self-contained HTML file. Dark theme, markdown rendering, syntax highlighting, per-tool formatting, collapsible thinking blocks, expandable tool outputs, and keyboard shortcuts (Ctrl+T, Ctrl+O). Supports `-o` for custom output path and `--stdout` for piping.
- **`hook notify` subcommand**: Logs Claude Code hook events (Stop, Notification, PreToolUse, PostToolUse, SessionEnd) to `/tmp/clotilde/<session-id>.events.jsonl` for debugging. Opt-in only, not registered by default setup.

### Changed

- **Session-reading commands**: `list`, `resume`, `inspect`, `delete`, `stats`, and `export` show friendly "no sessions found" messages instead of "not in a clotilde project" errors.
- **Dashboard**: Opens in any directory (auto-creates session storage). Empty session list is handled gracefully.

### Deprecated

- **`init` command**: Replaced by `setup`. Still works but prints a deprecation notice.

### Removed

- **`context.md` file**: The deprecated global context file (`.claude/clotilde/context.md`) has been removed. Use the `--context` flag instead.
- **Auto-created `config.json`**: Project-level config is no longer created automatically. Profiles still work if the file exists.

### Fixed

- **Dashboard start action**: Now auto-generates a session name and launches Claude directly instead of printing a placeholder message.
- **Dashboard fork action**: Now shows a session picker, auto-generates a fork name, and launches Claude instead of printing a placeholder message.

## [0.6.0] - 2026-03-08

### Changed

- **Auto-generated session names**: `start` no longer requires a name argument. When omitted, a date-prefixed name is generated automatically (e.g. `2026-03-08-happy-fox`). The `incognito` command uses the same `YYYY-MM-DD-adjective-noun` format.

## [0.5.0] - 2026-02-23

### Added

- **Global profiles**: Profiles can now be defined in `~/.config/clotilde/config.json` (respects `$XDG_CONFIG_HOME`). Global profiles are available in all projects. Project-level profiles take precedence over global ones when names collide. CLI flags still override both.
- **`--context` flag**: Attach context to sessions (e.g. `--context "working on ticket GH-123"`). Available on `start`, `incognito`, `fork`, and `resume` commands. Context is stored in session metadata and automatically injected into Claude at session start alongside the session name. Forked sessions inherit context from the parent unless overridden.
- **Session name injection**: The session name is now automatically output to Claude at session start via the SessionStart hook.

### Deprecated

- **`context.md` file**: Global context file (`.claude/clotilde/context.md`) is deprecated in favor of the `--context` flag. It will be removed in 1.0.

## [0.4.0] - 2026-02-20

### Added

- **Session profiles**: New named presets in `.claude/clotilde/config.json` for grouping model, permissions, and output style settings. Use `--profile <name>` when creating sessions. CLI flags override profile values.
  - Example: `clotilde start my-session --profile quick` applies the "quick" profile, then allows CLI flags to override individual settings
  - Profiles can contain: `model`, `permissionMode`, `permissions` (allow/deny/ask/additionalDirectories/defaultMode), and `outputStyle`

### Removed

- **Implicit global defaults**: Removed `model` and `permissions` from top-level config. Use profiles instead for explicit, named presets.

### Changed

- **Config structure**: `Config` now has `profiles` (map of Profile) instead of `DefaultModel`/`DefaultPermissions` fields

## [0.3.1] - 2026-02-18

### Fixed

- **Empty session detection with symlinks:** Sessions were incorrectly detected as empty (and auto-removed) when the project path involved symlinks. The transcript path saved by the SessionStart hook is now used for detection instead of recomputing it from the clotilde root, which could resolve to a different path than what Claude Code uses.

## [0.3.0] - 2026-02-17

### Added

- **Permission mode shortcuts**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask` as shorthand for `--permission-mode <value>` on `start`, `incognito`, `resume`, and `fork` commands
- **`--fast` composite preset**: Sets `--model haiku` and `--effort low` in a single flag for quick, low-cost sessions
- Conflict detection for mutually exclusive shorthand flags (e.g., `--accept-edits` + `--yolo`, or `--fast` + `--model`)

### Fixed

- **Ghost session cleanup:** Sessions created with `start` or `fork` are automatically removed if the user exits Claude Code without sending any messages (no transcript created)

### Changed

- **`start` command**: Instead of failing when a session with the same name exists, now prompts the user to resume it (in TTY mode) or suggests `clotilde resume <name>` (in non-TTY mode)
- **`resume` command refactored** from global variable to factory function (`newResumeCmd()`), enabling flag registration and consistent test isolation

## [0.2.0] - 2025-12-04

### Changed

- **Context system simplified:** Removed session-specific context support. Now only supports global context (`.claude/clotilde/context.md`)
- **Context source header:** Global context now includes a header indicating its source file, making it easier for Claude to know where to update context
- **Fork behavior:** Forks no longer inherit context from parent sessions (only settings and system prompt)
- **Documentation:** Updated docs to be worktree-agnostic (context works with or without git worktrees)

### Removed

- `LoadContext()` and `SaveContext()` methods from session store
- Session-specific `context.md` files (no longer created or copied during fork)

### Fixed

- Goreleaser archive configuration: Split into separate unix (tar.gz) and windows (zip) configurations for clearer build output

## [0.1.0] - 2025-12-02

Initial release of Clotilde - named session management for Claude Code.

### Added

**Core Features:**
- Named sessions with human-friendly identifiers (vs UUIDs)
- Session forking to explore different approaches
- Incognito sessions (auto-delete on exit) 👻
- Custom model and system prompt support per session
- System prompt replacement (replace Claude's default entirely)
- Two-level context system (global + session-specific)
- Persistent permission settings per session
- Pass-through flags support (forward arbitrary Claude Code flags)
- Full session cleanup (removes session data and Claude Code transcripts)
- Shell completion for bash, zsh, fish, and PowerShell

**Commands:**
- `init` - Initialize clotilde in a project
- `start` - Start new named sessions
- `incognito` - Start incognito sessions (auto-delete on exit)
- `resume` - Resume existing sessions
- `list` - List all sessions (table format)
- `inspect` - Show detailed session information with excerpts
- `fork` - Fork sessions (including incognito forks)
- `delete` - Delete sessions and associated data

**Enhancements:**
- Command aliases for common operations
- Table-formatted session list
- Inspect shows 200-char excerpts for prompts and context
- Humanized file sizes in inspect output
- System prompt content display in inspect
- Hide empty Settings section when no settings configured
- Support for `/compact` and `/clear` operations via unified SessionStart hook

**Documentation:**
- README with installation and usage guide
- CONTRIBUTING.md for contributors
- GitHub issue/PR templates
