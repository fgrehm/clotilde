# Clotilde Roadmap

A power-user companion for Claude Code.

## Known Limitations

- Incognito cleanup only runs on normal exit (not SIGKILL/crashes)
- `/compact` UUID tracking is defensive (Claude Code doesn't currently create new UUIDs for it)
- `/rename` inside a session is detected post-exit (transcript scan); if the new name fails slug validation, clotilde skips it silently with a stderr warning
- Zellij tab status integration blocked by Zellij limitations (see [investigation notes](zellij-tab-status.md))

## Tour Follow-ups

- **Debug/verbose mode for tour generation**: Show all stream-json events Claude emits during `tour generate` (tool inputs/outputs, assistant text blocks, result events). Currently only tool call summaries are shown. Could be `--verbose` or `CLOTILDE_TOUR_DEBUG=1`. Would help diagnose extraction failures and prompt issues.
- **Tour regeneration**: `tour generate --name existing` should detect an existing tour and offer to overwrite or diff.
- **Tour editing**: `tour edit <name>` to open the tour file in `$EDITOR` with validation on save.
- **Custom generation prompts**: Allow users to provide their own prompt template or append instructions (e.g. "focus on error handling patterns", "use a casual tone").

## Future Ideas

- **Session stats**: Record session statistics (turns, time, tokens, tool usage) to daily JSONL files on session end. Opt-in via `clotilde setup --stats`. [Spec](specs/sessionend-stats.md)
- **Zellij tab status**: Rename tab with emoji status during sessions. Blocked by `rename-tab` targeting focused tab and plugin off-by-one bug. Revisit when Zellij adds `--tab-index` support ([investigation](zellij-tab-status.md))
- **`adopt` command**: Register existing Claude Code sessions into Clotilde (deferred from `/branch` detection work — hook covers the common case)
- **Consolidate test framework**: Migrate to a single testing approach (standard `testing` or Ginkgo). Currently mixed: some packages use Ginkgo (`cmd/`, `internal/claude/hooks_test.go`), others use standard `testing` (`internal/claude/transcript_test.go`, `internal/claude/stats_file_test.go`). Both work with `go test` but the inconsistency adds friction.
- **Session search**: Full-text search across transcripts
- **Context templates**: Dynamic context (git branch, ticket info)
- **Auto-update**: `clotilde update` command that checks for a newer release on GitHub and replaces the binary in-place. Could also support `clotilde setup --auto-update` to register a periodic background check.
- **Session tags**: Organize with labels
- **Bulk operations**: Multi-select for batch delete
