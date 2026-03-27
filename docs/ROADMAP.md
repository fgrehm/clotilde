# Clotilde Roadmap

A power-user companion for Claude Code.

## Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)
- Zellij tab status integration blocked by Zellij limitations (parked on `zellij-tab-status` branch)

## Tour Follow-ups

- **Debug/verbose mode for tour generation**: Show all stream-json events during `tour generate` (tool inputs/outputs, assistant text, result events). Currently only tool call summaries are shown. Could be `--verbose` or `CLOTILDE_TOUR_DEBUG=1`.
- **Tour regeneration**: `tour generate --name existing` should detect an existing tour and offer to overwrite or diff.
- **Tour editing**: `tour edit <name>` to open the tour file in `$EDITOR` with validation on save.
- **Custom generation prompts**: Allow users to provide their own prompt template or append instructions.

## Future Ideas

- **Zellij tab status**: Rename tab with emoji status during sessions. Blocked by `rename-tab` targeting focused tab and plugin off-by-one bug. Revisit when Zellij adds `--tab-index` support.
- **`adopt` command**: Register existing Claude Code sessions into Clotilde.
- **Session search**: Full-text search across transcripts.
- **Context templates**: Dynamic context (git branch, ticket info).
- **Auto-update**: `clotilde update` command that checks for a newer release on GitHub and replaces the binary in-place.
- **Session tags**: Organize sessions with labels.
- **Bulk operations**: Multi-select for batch delete.
