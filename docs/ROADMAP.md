# Clotilde Roadmap

Named session management for Claude Code.

## Current Status: v0.2.0

Core functionality complete and tested.

### What's Working

- **Commands**: init, start, resume, list, inspect, fork, delete, incognito
- **Features**: Named sessions, forking, incognito mode, system prompts, permissions, global context system
- **TUI**: Dashboard, session picker, confirmation dialogs, styled output
- **Distribution**: Cross-platform binaries via goreleaser

### Recent Changes (v0.2.0)

- **Simplified context system**: Now uses only global context (`.claude/clotilde/context.md`), removed session-specific context
- **Context source headers**: Makes it easier for Claude to know where to update context
- **Fork behavior**: Forks inherit settings and system prompt (global context is shared across all sessions)

### Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)

## Future Ideas

- **Profiles**: Save session configs as reusable templates
- **Session search**: Full-text search across transcripts
- **Session export**: Export conversations to markdown
- **Context templates**: Dynamic context (git branch, ticket info)
- **Session tags**: Organize with labels
- **Bulk operations**: Multi-select for batch delete
