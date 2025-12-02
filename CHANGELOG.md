# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
