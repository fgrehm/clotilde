# Zellij Tab Status: Investigation Notes

**Date:** 2026-03-11
**Branch:** `zellij-tab-status`
**Status:** Parked. Blocked on Zellij not exposing a CLI to rename a tab by index.

## What We Built

A full implementation of emoji-prefixed Zellij tab renaming driven by Claude Code hook
events. The code is complete and tested but cannot be shipped because `zellij action
rename-tab` always targets the focused tab, not the tab where the Claude process is
running.

### What was implemented (then reverted)

- `internal/notify/emoji.go` -- `EmojiForEvent(hookEventName, payload)` mapping all
  hook event types to emoji prefixes:
  - Stop -> `checkmark`
  - Notification (permission_prompt) -> `warning`
  - Notification (idle_prompt) -> `zzz`
  - PreToolUse: Bash -> `lightning`, Read/Grep/Glob/ToolSearch -> `book`, Write/Edit/MultiEdit -> `pencil`, Task -> `shuffle`, WebSearch/WebFetch -> `globe`, other -> `gear`
  - PostToolUse -> `thinking face`
  - SessionEnd -> `""` (empty, restore signal)

- `internal/notify/zellij.go` -- `TabRenamer` interface + `ZellijPipeRenamer` impl
  that sends `zellij pipe --name change-tab-name -- {"pane_id": "$ZELLIJ_PANE_ID",
  "name": "emoji session-name"}` with a 2s timeout.

- `cmd/hook_notify.go` -- wired up: check `$ZELLIJ` + `zellijTabStatus` config flag,
  resolve session name (env var -> env file -> reverse UUID lookup), call renamer.

- `internal/config/config.go` -- `ZellijTabStatus bool` field on `Config`.

- `cmd/setup.go` -- `--zellij-tab-status` flag that saves the config opt-in and prints
  plugin install instructions.

- README -- "Zellij Tab Status (experimental)" section with setup instructions.

All code was covered by tests. The `TabRenamer` interface allows test injection via
`cmd.NotifyTabRenamer`.

## Why It Doesn't Work

### Attempt 1: `zellij action rename-tab`

Renames the currently focused tab, not the tab containing the calling process. If you
switch to another tab while Claude is working, that tab gets renamed instead.

No workaround: `rename-tab` has no flag to target a specific tab by index.

- Open Zellij issue: https://github.com/zellij-org/zellij/issues/4602
- PR adding `--tab-index`: pending as of 2026-03-11, not yet in 0.43.1

### Attempt 2: `zellij pipe` + zellij-tab-name plugin

The [zellij-tab-name plugin](https://github.com/Cynary/zellij-tab-name) by Cynary
accepts pipe messages `{"pane_id": "N", "name": "..."}` and renames the tab containing
pane N. This targets the right tab regardless of focus.

**Problem:** The plugin is consistently off-by-one in multi-tab layouts.

Confirmed via `zellij action dump-layout`: pane 5 is visually in tab 2, but
`zellij pipe --name change-tab-name -- {"pane_id": "5", "name": "TEST"}` renames tab 3.
Every session tested showed the same +1 offset.

Hypothesis: the plugin's "stable IDs" mechanism assigns internal tab IDs based on the
order it receives tab events at startup. With layouts that have plugin panes (tab-bar,
status-bar), the pane ID numbering in `ZELLIJ_PANE_ID` (which includes all panes) gets
out of sync with the plugin's internal position mapping (which may only count terminal
panes).

`use_stable_ids: false` in the payload made it worse (no rename at all).

This is likely related to: https://github.com/zellij-org/zellij/issues/3535

**Additional issue:** `zellij pipe` blocks until the plugin handles the message. If the
plugin is not loaded, the call hangs indefinitely. Our implementation wraps it in a 2s
`context.WithTimeout` to avoid blocking hooks.

## What To Do When Revisiting

### Path A: Wait for `--tab-index` in `zellij action rename-tab`

Watch https://github.com/zellij-org/zellij/issues/4602. When merged, change
`ZellijPipeRenamer.RenameTab()` to:

```go
cmd := exec.CommandContext(ctx, "zellij", "action", "rename-tab", name, "--tab-index", tabIndex)
```

Where `tabIndex` is captured at session start (see below).

**Capturing tab index at startup:** `$ZELLIJ_TAB_INDEX` does not exist as an env var.
To get it, parse `zellij action dump-layout` at `clotilde start`/`resume` time, find the
tab where `focus=true`, count its position (1-indexed), and stash it in a temp file at
`/tmp/clotilde/<session-id>.tab-index`. The notify hook reads this file.

### Path B: Fix the zellij-tab-name plugin offset bug

If the +1 offset is reproducible and deterministic, it might be fixable upstream or
workable-around by subtracting 1 from `ZELLIJ_PANE_ID` before sending. Needs more
investigation to confirm whether the offset is always exactly 1 and whether it holds
across different layout configurations.

### Path C: Pane name instead of tab name

`zellij action rename-pane` shows the pane title in the tab bar in some themes. Same
focused-pane bug as `rename-tab`, but worth checking if it has `--pane-id` support.

## Environment Details

- Zellij: `0.43.1` (latest as of 2026-03-11)
- zellij-tab-name plugin: `v0.4.2`
- Plugin configured in `~/.config/zellij/config.kdl` under `load_plugins`
- The plugin must be loaded at session start (requires new Zellij session after config change)

## Code Location

The reverted implementation lives in git history on the `zellij-tab-status` branch:

- `43d47f9` feat: rename Zellij tab with emoji status on hook events
- `ac0c5a0` fix: use zellij pipe to rename tabs by pane ID
- `43c5c06` feat: add setup --zellij-tab-status opt-in flag

Restore with:
```bash
git checkout 43c5c06 -- cmd/hook_notify.go cmd/hook_test.go \
  internal/notify/emoji.go internal/notify/emoji_test.go \
  internal/notify/zellij.go internal/notify/zellij_test.go \
  internal/config/config.go internal/config/load.go \
  cmd/setup.go README.md
```
