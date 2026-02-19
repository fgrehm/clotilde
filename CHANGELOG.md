# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Global config file**: New `.claude/clotilde/config.json` file to set project-wide defaults. Currently supports `model` setting to specify the default Claude model for new sessions (used when `--model` flag is not provided).

### Fixed

- **Global config not applied to sessions**: The `model` setting in `.claude/clotilde/config.json` was not being read when creating new sessions. Now `clotilde start` and `clotilde incognito` properly use the configured default model as a fallback when `--model` flag is not specified.

### Changed

- **JSON struct tag for config**: Fixed `Config.DefaultModel` struct tag from `"defaultModel"` to `"model"` to match expected config file format.

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
