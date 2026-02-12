# Lazy Directory Creation for Worktree Support

## Problem

When using multiple git worktrees, `.claude/settings.json` (containing hooks) may be shared or copied across worktrees, but `.claude/clotilde/sessions/` is worktree-local state. Currently `clotilde init` creates both the hook AND the directory structure, meaning every new worktree requires re-running `clotilde init` even though the hook is already there.

## Proposal

`clotilde init` only registers the hook in `.claude/settings.json`. The `.claude/clotilde/` directory structure gets created lazily when first needed (e.g., `clotilde start`).

## Changes

### 1. `clotilde init` becomes purely about hook management

- Register/update the `SessionStart` hook in `.claude/settings.json`
- Remove the call to `EnsureClotildeStructure()`
- Re-running is still idempotent (update hooks if needed)

### 2. `EnsureClotildeStructure()` called lazily

Call `EnsureClotildeStructure()` at the entry point of commands that need the directory:

- `start`
- `resume`
- `fork`
- `incognito`
- `delete`
- `list`
- `inspect`

The function is already idempotent, so calling it every time is safe.

### 3. `IsInitialized()` checks for the hook, not the directory

Change from "does `.claude/clotilde/` exist?" to "is the hook registered in `.claude/settings.json`?".

### 4. `config.json` created lazily

Currently created during init inside `.claude/clotilde/`. Move creation into `EnsureClotildeStructure()` so it happens on demand (it already lives there, just needs to be called from the right places).

### 5. `FindClotildeRoot()` behavior

Currently this is the "is clotilde initialized?" gate. With lazy creation, it can't be the sole gate anymore. Options:

- Have it create the directory if missing (when the hook exists), or
- Replace its usage with a two-step check: verify hook exists, then ensure structure
