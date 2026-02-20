# Clotilde Roadmap

Named sessions, profiles, and context management for Claude Code.

## Current Status: v0.4.0 (unreleased)

Core functionality complete and tested. Recent work focused on session profiles and output styles.

### What's Working

- **Commands**: init, start, resume, list, inspect, fork, delete, incognito, completion
- **Features**: Named sessions, forking, incognito mode, system prompts, permissions, global context, session profiles, output styles
- **Shorthand flags**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast`
- **TUI**: Dashboard, session picker, confirmation dialogs, styled output
- **Distribution**: Cross-platform binaries via goreleaser

### v0.4.0 Highlights

- **Session profiles**: Named presets in `config.json` for model, permissions, and output style. Apply with `--profile <name>`.
- **Output styles**: Per-session output style via `--output-style` and `--output-style-file`. Supports built-in styles, project/user styles, and custom inline content.
- **Global config**: Expanded to support project-wide permissions defaults

### v0.3.0 Highlights

- **Permission mode shortcuts**: `--accept-edits`, `--yolo`, `--plan`, `--dont-ask` on all session commands
- **`--fast` preset**: `--model haiku` + `--effort low` in a single flag
- **Conflict detection**: Mutually exclusive shorthand flags produce clear error messages
- **`start` resumes existing sessions**: Prompts to resume instead of failing on duplicate names
- **Ghost session cleanup**: Empty sessions (no messages sent) are automatically removed

### Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)
- `/fork` slash command inside a Clotilde session creates an untracked fork (see [slash-fork-handling spec](specs/slash-fork-handling.md))

## Future Ideas

- **`/fork` slash command support**: Auto-detect and register forks created via Claude Code's `/fork` command ([spec](specs/slash-fork-handling.md))
- **`adopt` command**: Register existing Claude Code sessions into Clotilde
- **Session search**: Full-text search across transcripts
- **Session export**: Export conversations to markdown
- **Context templates**: Dynamic context (git branch, ticket info)
- **Session tags**: Organize with labels
- **Bulk operations**: Multi-select for batch delete
