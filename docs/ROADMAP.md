# Clotilde Roadmap

A power-user companion for Claude Code.

## Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)
- Zellij tab status integration blocked by Zellij limitations (parked on `zellij-tab-status` branch)

## Future Ideas

- **Zellij tab status**: Rename tab with emoji status during sessions. Blocked by `rename-tab` targeting focused tab and plugin off-by-one bug. Revisit when Zellij adds `--tab-index` support.
- **`adopt` command**: Register existing Claude Code sessions into Clotilde.
- **Session search**: Full-text search across transcripts.
- **Context templates**: Dynamic context (git branch, ticket info).
- **Auto-update**: `clotilde update` command that checks for a newer release on GitHub and replaces the binary in-place.
- **Session tags**: Organize sessions with labels.
- **Bulk operations**: Multi-select for batch delete.
