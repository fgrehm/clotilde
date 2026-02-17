# Clotilde Roadmap

Named session management for Claude Code.

## Current Status: v0.3.0

Core functionality complete and tested. Working on quality-of-life improvements.

### What's Working

- **Commands**: init, start, resume, list, inspect, fork, delete, incognito
- **Features**: Named sessions, forking, incognito mode, system prompts, permissions, global context system
- **Shorthand flags**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast`
- **TUI**: Dashboard, session picker, confirmation dialogs, styled output
- **Distribution**: Cross-platform binaries via goreleaser

### v0.3.0 Highlights

- **Permission mode shortcuts**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask` on all session commands
- **`--fast` preset**: `--model haiku` + `--effort low` in a single flag
- **Conflict detection**: Mutually exclusive shorthand flags produce clear error messages
- **`start` resumes existing sessions**: Prompts to resume instead of failing on duplicate names
- **Ghost session cleanup**: Empty sessions (no messages sent) are automatically removed

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
