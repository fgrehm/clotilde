# Clotilde Roadmap

A power-user companion for Claude Code.

## Current Status: v0.8.1

### What's Working

- **Commands**: setup, start, resume, list, inspect, fork, delete, incognito, export, completion
- **Features**: Named sessions, forking, incognito mode, system prompts, permissions, session context, session profiles (global + project), output styles, session export (HTML)
- **Shorthand flags**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast`
- **TUI**: Dashboard (with start/fork actions), session picker, confirmation dialogs, styled output
- **Debugging**: `hook notify` subcommand logs hook events to JSONL (opt-in)
- **Distribution**: Cross-platform binaries via goreleaser

### v0.7.0 Highlights

- **Global installation**: `clotilde setup` registers hooks in `~/.claude/settings.json`. No more per-project `init` required. Session directories are created automatically on first use.
- **`init` deprecated**: Still works but prints a deprecation notice directing to `setup`.
- **`export` command**: `clotilde export <name>` renders a session transcript into a self-contained HTML file with dark theme, syntax highlighting, collapsible thinking blocks, and keyboard shortcuts.
- **Dashboard actions**: Start and fork actions now work end-to-end from the dashboard TUI.
- **Hook event logging**: `clotilde hook notify` logs Claude Code events to `/tmp/clotilde/<session-id>.events.jsonl` for debugging. Opt-in, not registered by default setup.

### Previous Releases

- **v0.6.0**: Auto-generated session names (`YYYY-MM-DD-adjective-noun` format)
- **v0.5.0**: Global profiles, `--context` flag, session name injection via hooks
- **v0.4.0**: Session profiles, removed implicit global defaults
- **v0.3.0**: Permission mode shortcuts, `--fast` preset, ghost session cleanup
- **v0.2.0**: Simplified context system, goreleaser fixes
- **v0.1.0**: Initial release

### Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)
- `/fork` slash command inside a Clotilde session creates an untracked fork (see [slash-fork-handling spec](specs/slash-fork-handling.md))
- Zellij tab status integration blocked by Zellij limitations (see [investigation notes](zellij-tab-status.md))

## Future Ideas

- **Session stats**: Record session statistics (turns, time, tokens, tool usage) to daily JSONL files on session end. Opt-in via `clotilde setup --stats`. [Spec](specs/sessionend-stats.md)
- **Zellij tab status**: Rename tab with emoji status during sessions. Blocked by `rename-tab` targeting focused tab and plugin off-by-one bug. Revisit when Zellij adds `--tab-index` support ([investigation](zellij-tab-status.md))
- **`/fork` slash command support**: Auto-detect and register forks created via Claude Code's `/fork` command ([spec](specs/slash-fork-handling.md))
- **`adopt` command**: Register existing Claude Code sessions into Clotilde
- **Consolidate test framework**: Migrate to a single testing approach (standard `testing` or Ginkgo). Currently mixed: some packages use Ginkgo (`cmd/`, `internal/claude/hooks_test.go`), others use standard `testing` (`internal/claude/transcript_test.go`, `internal/claude/stats_file_test.go`). Both work with `go test` but the inconsistency adds friction.
- **Session search**: Full-text search across transcripts
- **Context templates**: Dynamic context (git branch, ticket info)
- **Auto-update**: `clotilde update` command that checks for a newer release on GitHub and replaces the binary in-place. Could also support `clotilde setup --auto-update` to register a periodic background check.
- **Session tags**: Organize with labels
- **Bulk operations**: Multi-select for batch delete
