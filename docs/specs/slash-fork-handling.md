# Plan: First-class `/fork` slash command support

## Context

Claude Code has a `/fork` slash command that creates a fork mid-session:

```
❯ /fork
  ⎿  Forked conversation. You are now in the fork.
     To resume the original: claude -r 989a56d3-...
```

When this runs inside a Clotilde session, the fork is invisible to Clotilde. The user gets a raw UUID with no name tracking, no parent/child metadata, and no way to `clotilde resume` the fork. The existing `clotilde fork` command works by setting env vars (`CLOTILDE_FORK_NAME`, etc.) before invoking Claude, but `/fork` bypasses that entirely.

## Step 0: Verify `/fork` hook behavior (manual test)

**Must confirm before implementing.** We need to know:
- Does `/fork` trigger a SessionStart hook?
- If so, what `source` field? (`"resume"`? `"startup"`? `"fork"`?)
- What `session_id` does the hook receive? (The fork's new UUID?)

**Test procedure**: Run `/fork` inside a Clotilde session that has `context.md` set up. If the "Clotilde session context source" banner appears after forking, the hook fires. Note the source from debug output (`clotilde resume <name> -- --debug hooks`).

**If the hook does NOT fire**: Skip Steps 1-3, rely entirely on the `adopt` command (Step 4). Consider outputting a tip in session context: "If you use /fork, run `clotilde adopt <uuid> <name>` to register the fork."

## Step 1: Auto-detect `/fork` in the SessionStart hook

**File**: `cmd/hook_sessionstart.go` (`handleResume`, lines 94-125)

**Detection signal**: When all three conditions are true:
1. `CLOTILDE_SESSION_NAME` is set (we're in a known session)
2. `CLOTILDE_FORK_NAME` is NOT set (not a `clotilde fork` invocation)
3. The incoming `session_id` differs from the stored UUID for that session AND is not in `previousSessionIds`

This means a new UUID appeared that we didn't expect, which means `/fork`.

**Changes to `handleResume()`**:

```go
forkName := os.Getenv("CLOTILDE_FORK_NAME")
if forkName != "" {
    // Existing path: explicit fork via clotilde fork
    registerFork(...)
    sessionName = forkName
} else if sessionName != "" {
    // NEW: Check for in-session /fork
    detectedFork := detectSlashFork(clotildeRoot, store, sessionName, hookData)
    if detectedFork != "" {
        sessionName = detectedFork
    }
}
```

**New `detectSlashFork()` function**:
- Load parent session by `CLOTILDE_SESSION_NAME`
- Compare `hookData.SessionID` against `parent.Metadata.SessionID` and `parent.Metadata.PreviousSessionIDs`
- If no match: this is a `/fork`
- Auto-generate fork name (see Step 2)
- Create fork session: `NewForkedSession(forkName, parentName)`, set `SessionID` and `TranscriptPath`
- Copy `settings.json` and `system-prompt.md` from parent session dir to fork dir
- Write fork name to `CLAUDE_ENV_FILE` (critical for subsequent `/clear` or `/fork` inside the fork)
- Output notice to stderr: `"Clotilde: registered fork as '<name>' (from '<parent>')\n"`
- Return fork name

**Race condition**: If `/fork` is run twice rapidly, `store.Create()` may fail on name collision. Wrap in retry loop (max 3 attempts) that regenerates the name on failure.

## Step 2: Auto-naming strategy

**File**: `cmd/hook_sessionstart.go` (new `generateForkName()` function)

Pattern: `{parent}-fork-1`, `{parent}-fork-2`, etc.

- List existing sessions, try sequential suffixes starting at 1
- If parent name is too long (would exceed `MaxNameLength` of 64 chars with suffix), truncate parent name to fit
- Fallback: random name via `util.GenerateUniqueRandomName()` if sequential names are exhausted (> 100 forks)

Rationale for `{parent}-fork-N` over random names: `clotilde list` immediately shows parentage in the name. The `ParentSession` metadata field records it regardless, but name-based linkage is more ergonomic.

## Step 3: Fix session name resolution after auto-fork

**Problem**: After `/fork` auto-detection, the process env var `CLOTILDE_SESSION_NAME` still holds the parent name. The hook writes the fork name to `CLAUDE_ENV_FILE`, but `getSessionName()` (line 178) checks `CLOTILDE_SESSION_NAME` first (Priority 1). If the user runs `/clear` inside the fork, the hook would resolve to the parent name and corrupt the parent's metadata.

**Fix in `getSessionName()`**: Add UUID validation to the resolution logic:

```go
// Priority 1: CLOTILDE_SESSION_NAME env var
if name := os.Getenv("CLOTILDE_SESSION_NAME"); name != "" {
    // Validate that the UUID matches (handles /fork case where env var is stale)
    if sess, err := store.Get(name); err == nil {
        if sess.Metadata.SessionID == hookData.SessionID || containsID(sess.Metadata.PreviousSessionIDs, hookData.SessionID) {
            return name, nil
        }
    }
    // UUID doesn't match, fall through to Priority 2
}

// Priority 2: CLAUDE_ENV_FILE
if name := readSessionNameFromEnvFile(); name != "" {
    return name, nil
}

// Priority 3: Reverse UUID lookup
return findSessionByUUID(store, hookData.SessionID)
```

This ensures that after a `/fork`, when the env var points to the parent but the UUID is the fork's, the resolution falls through to the env file (which has the fork name).

**Note**: This change also needs to apply in `handleResume` and `handleStartup` where `CLOTILDE_SESSION_NAME` is read directly (lines 71, 95). Those paths should use the same validated resolution.

## Step 4: `adopt` command (fallback / general-purpose)

**New file**: `cmd/adopt.go`

```
clotilde adopt <uuid> [name] [--parent <session>]
```

For when the hook doesn't catch a fork (or for adopting any standalone Claude Code session):

```bash
# After /fork prints: "To resume the original: claude -r 989a56d3-..."
# The UUID shown is the ORIGINAL session. The current fork has a new UUID.
# User can find the fork UUID via `claude sessions list`

clotilde adopt <fork-uuid> my-fork-name --parent auth-feature

# Or auto-name:
clotilde adopt <fork-uuid>
```

**Implementation**:
- Validate UUID isn't already tracked (check all sessions' current + previous IDs)
- Validate name (or auto-generate via `util.GenerateUniqueRandomName()`)
- Create session with `NewSession(name, uuid)`, set `ParentSession` and `IsForkedSession` if `--parent` provided
- Create empty `settings.json`

**Register in**: `cmd/root.go` (both `initRootCmd()` and `NewRootCmd()`)

## Step 5: Tests

**Hook auto-detection tests** (extend `cmd/hook_test.go`):
1. In-session fork detected: `CLOTILDE_SESSION_NAME` set, no `CLOTILDE_FORK_NAME`, different UUID → new session created with `{parent}-fork-1`, `IsForkedSession: true`, `ParentSession` set
2. Normal resume not misdetected: same UUID as stored → no new session
3. Known previous UUID not misdetected: UUID in `previousSessionIds` → no new session
4. Sequential naming: existing `parent-fork-1` → creates `parent-fork-2`
5. Explicit `clotilde fork` still works (no regression): both env vars set → existing `registerFork` path taken
6. Fork of a fork: `CLOTILDE_SESSION_NAME` = `parent-fork-1`, different UUID → creates `parent-fork-1-fork-1`
7. Session name resolution after fork: `CLOTILDE_SESSION_NAME` points to parent, UUID matches fork → resolves to fork name

**Adopt command tests** (new `cmd/adopt_test.go`):
1. Adopt with explicit name
2. Adopt with auto-generated name
3. Adopt with `--parent` flag
4. Reject duplicate UUID
5. Reject duplicate name

## Step 6: Documentation

- **CLAUDE.md**: Update "Fork registration" section to document auto-detection path and `adopt` command
- **README.md**: Mention `/fork` support in the forking section, document `adopt` command

## Files to modify

- `cmd/hook_sessionstart.go` - Core detection logic, session name resolution fix
- `cmd/hook_test.go` - New test cases for auto-detection
- `cmd/adopt.go` - New command (new file)
- `cmd/adopt_test.go` - Tests for adopt (new file)
- `cmd/root.go` - Register adopt command
- `CLAUDE.md` - Update fork registration docs
- `README.md` - Document /fork support and adopt command

## Verification

1. Run `clotilde start test-session`, use `/fork` inside it, verify `clotilde list` shows the auto-registered fork
2. Inside the fork, run `/clear`, verify the fork's metadata is updated (not the parent's)
3. `clotilde inspect <fork-name>` shows correct parent and UUID
4. `clotilde resume <fork-name>` works
5. `clotilde delete <fork-name>` cleans up correctly
6. `clotilde adopt <uuid> my-name --parent test-session` works
7. `make test` passes
