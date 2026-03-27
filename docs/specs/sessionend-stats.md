# SessionEnd Stats Recording

**Date:** 2026-03-13
**Status:** Design complete, not yet implemented

## Goal

Record session statistics (turns, time, tokens, tool usage) to a daily JSONL file
when a Claude Code session ends. Enables end-of-day summaries of AI usage across
sessions. Opt-in via `clotilde setup --stats`.

---

## Current State

**What exists:**
- `ParseTranscriptStats` (`internal/claude/transcript.go:123`) returns per-transcript stats:
  `Turns`, `ActiveTime`, `TotalTime`, `FirstMessage`, `LastMessage`, `AvgResponseTime`
- `SessionEnd` is a recognized hook type in `HookConfig` (`internal/claude/hooks.go:27`)
- `GenerateNotifyHookConfig` registers `SessionEnd` pointing to `clotilde hook notify`
- `GenerateHookConfig` does NOT register `SessionEnd`
- `getSessionName` three-level fallback lives in `cmd/hook_sessionstart.go:194`
- `GlobalConfigPath` (`internal/config/paths.go:75`) shows the XDG pattern to follow

**What needs to be built:**
- `MergeTranscriptStats` -- merge stats across multiple transcripts
- `allTranscriptPaths` -- build ordered list of transcript paths for a session
- `cmd/session_helpers.go` -- shared helpers (does not exist yet)

## Design

### Stats record schema

One JSON object per line in the daily JSONL file:

```json
{
  "session_name": "my-feature",
  "session_id": "uuid",
  "project_path": "/home/user/projects/myapp",
  "turns": 12,
  "active_time_s": 3600,
  "total_time_s": 7200,
  "input_tokens": 45000,
  "output_tokens": 12000,
  "cache_creation_tokens": 5000,
  "cache_read_tokens": 30000,
  "models": ["claude-sonnet-4-5-20250929"],
  "tool_uses": {"Bash": 15, "Read": 22, "Edit": 8, "Write": 3},
  "prev_turns": 5,
  "prev_active_time_s": 1200,
  "prev_total_time_s": 3000,
  "prev_input_tokens": 20000,
  "prev_output_tokens": 5000,
  "ended_at": "2026-03-13T18:00:00Z"
}
```

**Identity fields:**
- `session_name`: from env var / env file / UUID reverse lookup. Empty string if not found.
- `session_id`: from stdin payload.
- `project_path`: clotilde root's grandparent (`.claude/clotilde` -> two levels up). Empty if not resolvable.

**Activity metrics (cumulative across all transcripts):**
- `turns`: human turns.
- `active_time_s`: sum of `ActiveTime`, truncated to integer seconds.
- `total_time_s`: `LastMessage - FirstMessage`, truncated to integer seconds.

**Token metrics (cumulative, from `message.usage` on assistant entries):**
- `input_tokens`: fresh input tokens (not cached).
- `output_tokens`: generated output tokens.
- `cache_creation_tokens`: tokens that created cache entries.
- `cache_read_tokens`: tokens served from cache.

**Usage breakdown:**
- `models`: deduplicated list of model IDs seen across all transcripts (from `message.model`
  on assistant entries). Order: first appearance.
- `tool_uses`: map of tool name to invocation count (from `message.content[]` blocks with
  `type: "tool_use"`). Only tools that were actually invoked are included.

**Delta fields (from most recent prior record for this `session_id`):**
- `prev_turns`, `prev_active_time_s`, `prev_total_time_s`: for activity delta.
- `prev_input_tokens`, `prev_output_tokens`: for token delta.
- All zero if no prior record found.

**Timestamp:**
- `ended_at`: RFC 3339 UTC timestamp of hook invocation.

The `prev_*` fields make each record self-contained for delta computation:
`turns this invocation = turns - prev_turns`. No cross-file lookups needed.

Note: `prev_*` fields are not tracked for `cache_creation_tokens`, `cache_read_tokens`,
`models`, or `tool_uses`. Cache tokens are a breakdown of input tokens (the delta on
`prev_input_tokens` covers it). Models and tool_uses are categorical, not summable.

### Stats file path

1. `$XDG_DATA_HOME/clotilde/stats/YYYY-MM-DD.jsonl`
2. `~/.local/share/clotilde/stats/YYYY-MM-DD.jsonl` (fallback)

Directory created on first write. File appended to (created if missing).

### Hook registration (opt-in)

Stats tracking is **opt-in**. Enabled during setup:

```
clotilde setup --stats        # enable stats tracking
clotilde setup --no-stats     # disable stats tracking (removes SessionEnd hook)
```

When `--stats` is passed, `GenerateHookConfig` includes the `SessionEnd` hook. Without it,
the hook is omitted. The preference is stored in global config
(`~/.config/clotilde/config.json`) as `"statsTracking": true`.

If neither flag is passed, `clotilde setup` prompts interactively:

```
Track session statistics (turns, tokens, tool usage)? [y/N]
```

Default is no. Stats can be enabled later by re-running `clotilde setup --stats`.

The `GenerateHookConfig` function reads the global config to decide whether to include the
SessionEnd hook:

```json
"SessionEnd": [{"hooks": [{"type": "command", "command": "clotilde hook sessionend"}]}]
```

No matcher -- fires for every session end.

### Command flow (`clotilde hook sessionend`)

1. Read JSON payload from stdin: `{"session_id": "...", "transcript_path": "...", "cwd": "...", "reason": "..."}`
2. Resolve session name via `resolveSessionName(hookData, store, true)` (full fallback).
3. **If clotilde root found:** load session from store, build list of all transcript paths
   (current + previous from `/clear` operations), merge stats across all transcripts.
4. **If clotilde root NOT found:** fall back to `transcript_path` from payload only.
   Stats will be partial; `session_name` and `project_path` will be empty.
5. Resolve project path: clotilde root's grandparent.
6. Look up the most recent prior record for this `session_id` via `FindLastRecord`.
   Populate `prev_*` fields from it (or zero if no prior record).
7. Write stats record to daily JSONL file.

### Resumed sessions and deduplication

A session can be resumed multiple times, producing multiple `SessionEnd` records (same day
or across days). Each record contains **cumulative** stats plus `prev_*` fields from the
prior record, enabling delta computation without cross-file lookups.

**Within a daily file:** Multiple records for the same `session_id` can coexist. The latest
record (last line) supersedes earlier ones for cumulative totals. Readers should deduplicate
by `session_id`, keeping the last occurrence.

**Across daily files:** A session resumed on Tuesday that was last ended on Monday will have
`prev_*` fields reflecting Monday's cumulative values. The delta (`turns - prev_turns`)
correctly represents Tuesday's activity.

**Consolidation:** `AppendStatsRecord` is append-only (no rewriting on write). A separate
`ConsolidateStatsFile(path)` function deduplicates a daily file, keeping only the last
record per `session_id`. It writes to a temp file in the same directory (`.YYYY-MM-DD.jsonl.tmp`)
then does an atomic `os.Rename` to the final path. This avoids data loss if the process
crashes mid-write.

Consolidation runs only when duplicates are detected: `ReadStatsFile` returns all records;
the caller checks if `len(records) != len(uniqueSessionIDs)` before calling consolidate.
This avoids unnecessary rewrites and the write-on-read concern with concurrent access.

Future option: a standalone `clotilde stats consolidate` command (not in this scope).

### Populating `prev_*` fields

When writing a new record, the hook needs the most recent prior record for the same
`session_id`. Lookup order:

1. Scan today's daily file (if it exists) for the last record matching `session_id`.
2. If not found, scan yesterday's file, then the day before, up to 7 days back.
3. If no prior record is found, `prev_*` fields are all zero (first record for this session).

The 7-day lookback window is a pragmatic bound. Sessions not touched in over a week will
get a zero baseline, which means their first record after the gap will show cumulative
totals as the delta. This is acceptable -- the alternative (scanning all files ever) is
not worth the complexity.

### Forked sessions

Forks are independent sessions with their own UUID, transcript, and name. No special
handling needed:

- Fork gets a pre-assigned UUID via `--session-id` before invocation. Its transcript file is
  separate from the parent's.
- Fork does NOT inherit `previousSessionIds` from the parent. Its transcript list is its
  own (only grows if the fork itself gets `/clear`'d).
- Fork has its own `session_name`, so dedup and `FindLastRecord` are naturally scoped.
- Parent and fork can end independently. Their stats records are fully isolated.

**Incognito sessions:** The session auto-deletes on exit, but the stats record would still
be written (the hook fires before cleanup). This is desirable -- usage tracking should
include throwaway sessions. The `session_name` will be the incognito session's name.

### `/clear` and `FindLastRecord`

After a `/clear`, the session gets a new UUID. The `session_id` in subsequent stats records
changes. `FindLastRecord` searches by `session_id` (UUID), so it won't find pre-/clear
records.

Effect: `prev_*` fields reset to zero after a `/clear`. The delta for the first record
after `/clear` equals the full cumulative total, overstating that invocation's work.

The cumulative stats are still correct (they merge all transcripts including previous UUIDs).
Only the per-invocation delta is wrong for one record. `/clear` is rare enough that this is
acceptable. A future improvement could search by (`session_name`, `project_path`) as a
fallback when UUID lookup finds nothing.

### `/branch` slash command interaction

Clotilde does not detect or track Claude Code's in-session `/branch` command. If a user
runs `/branch` inside a clotilde session, the resulting branch session lives outside
clotilde's tracking. Stats for the parent session are unaffected since the hook only
updates metadata for sessions it knows about (resolved via `CLOTILDE_SESSION_NAME` or
`CLAUDE_ENV_FILE`). Use `clotilde fork` for tracked forks.

### Exit behavior and signal safety

**User-visible message:** When the SessionEnd hook fires, print a brief message to stderr
so the user knows stats are being computed (avoids the impression of hanging):

```
clotilde: saving session stats...
```

On completion (or skip due to error): no further output. Keep it minimal.

**Signal handling:** The hook must survive interrupts during the file write. Mask SIGINT and
SIGTERM only around the critical section (the append), not the entire function. Transcript
parsing and `FindLastRecord` remain interruptible.

```go
// ... read payload, resolve session, parse transcripts, find prev record ...
// (all interruptible by Ctrl+C)

// Critical section: mask signals for the file write only
signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
err := claude.AppendStatsRecord(record)
signal.Reset(syscall.SIGINT, syscall.SIGTERM)
```

The write is a single `os.OpenFile` + `f.Write` + `f.Close` (sub-millisecond). No `defer`
needed for signal reset since the scope is trivially small.

If the hook is killed with SIGKILL (or power loss), `O_APPEND` semantics mean partial
writes are unlikely to corrupt existing records. A truncated last line in the JSONL file
is handled by readers (skip lines that fail JSON unmarshal).

### Crash recovery ("never ended" sessions)

If a session ends without the SessionEnd hook firing (power loss, SIGKILL, machine crash),
no stats record is written. When the session is later resumed, the SessionStart hook fires
with `source: "resume"`.

**Recovery logic (in SessionStart hook, `handleResume`):**

1. After resolving the session name, check if this session has a stats record for its most
   recent invocation by calling `FindLastRecord(session_id)`.
2. If no record is found AND the session's `lastAccessed` timestamp is older than the
   current time by more than a few minutes (i.e., this is a genuine resume, not a
   double-fire), write a **recovery record** using the transcript data available now.
3. The recovery record has `ended_at` set to the session's `lastAccessed` timestamp (best
   approximation of when it was last active). It is filed into the daily stats file for
   that date.
4. The `prev_*` fields for the recovery record follow the same lookup logic (may be zero
   if this was the first invocation).

This ensures that sessions interrupted by crashes still contribute to usage tracking. The
stats will be slightly stale (based on whatever was written to the transcript before the
crash) but that is better than a gap.

**Fast-path optimization:** Before doing any I/O for recovery, compare the session's
`lastAccessed` timestamp against the current time. If within 30 seconds, skip recovery
entirely (double-fire or immediate resume, no crash).

**Edge cases:**
- If the session was never started (empty transcript), skip recovery.
- If a record already exists (normal exit, then resume), skip recovery (no duplicate).
- Recovery runs inside the SessionStart hook, which already has access to the session store
  and clotilde root. The transcript parsing is the same code path as SessionEnd.

### Extended transcript parsing

`ParseTranscriptStats` needs to be extended to extract token and tool usage data from the
same scan pass. The transcript JSONL entries already contain this data:

- **Token usage:** On assistant entries, `message.usage` has `input_tokens`,
  `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.
- **Model:** On assistant entries, `message.model` has the full model ID.
- **Tool usage:** On assistant entries, `message.content` is an array; blocks with
  `type: "tool_use"` have a `name` field (e.g., "Bash", "Read", "Edit").

The existing `transcriptEntryForStats` struct needs additional fields. Since we already
scan every line, the overhead is just unmarshaling a few more fields per entry.

Extended `TranscriptStats` struct:

```go
type TranscriptStats struct {
    // Existing fields
    Turns           int
    FirstMessage    time.Time
    LastMessage     time.Time
    TotalTime       time.Duration
    ActiveTime      time.Duration
    AvgResponseTime time.Duration

    // New fields
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    Models              []string       // deduplicated, first-appearance order
    ToolUses            map[string]int // tool name -> invocation count
}
```

`ToolUses` must be initialized with `make(map[string]int)` in `ParseTranscriptStats`
before the scan loop (nil maps panic on write in Go). Same for `MergeTranscriptStats`
when creating the merged result.

**Content parsing strategy:** Keep `message.content` as `json.RawMessage` on the entry
struct. Only unmarshal into `[]struct{ Type, Name string }` for assistant entries to
extract tool_use names. This matches the existing `isHumanTurn` pattern (checks raw bytes
for user entries, avoids allocation for non-assistant entries).

### Multi-transcript merging

A session can have multiple transcripts from `/clear` operations. Each old UUID is stored in
`metadata.PreviousSessionIDs`. The merge logic:

1. Build an ordered list of transcript paths: one per previous UUID, then the current UUID.
   Paths computed via `claude.TranscriptPath(homeDir, clotildeRoot, uuid)`.
2. Parse each transcript individually with `ParseTranscriptStats`.
3. Merge:
   - Sum: `Turns`, `ActiveTime`, `InputTokens`, `OutputTokens`, `CacheCreationTokens`,
     `CacheReadTokens`, all `ToolUses` counts.
   - Min/max: earliest `FirstMessage`, latest `LastMessage`.
   - Deduplicate: `Models` (union, preserving first-appearance order).
   - Recompute: `TotalTime` = `LastMessage - FirstMessage`,
     `AvgResponseTime` = `ActiveTime / Turns`.

If `metadata.TranscriptPath` is set, use it for the current UUID's path (it may differ from
the computed path). For previous UUIDs, always compute the path.

### Error handling

- Unreadable transcripts: skip writing, log warning to stderr, exit 0.
- Unwritable stats file: log warning to stderr, exit 0.
- Hooks must not crash Claude Code's session end flow.
- Double-execution guard (same pattern as SessionStart) prevents duplicate records when
  both global and per-project hooks are registered.

---

## Implementation Plan

### 1. Extend `ParseTranscriptStats` and add multi-transcript helpers

**Modify `internal/claude/transcript.go`:**
- Extend `TranscriptStats` struct with `InputTokens`, `OutputTokens`,
  `CacheCreationTokens`, `CacheReadTokens`, `Models`, `ToolUses`.
- Extend `transcriptEntryForStats` to parse `message.usage`, `message.model`, and
  `message.content` (for tool_use blocks).
- Update `ParseTranscriptStats` to populate the new fields during the existing scan.
- Add `MergeTranscriptStats(stats []*TranscriptStats) *TranscriptStats` -- merges
  multiple stats into one (sum counts, min/max times, union models, sum tool uses).

**New file: `cmd/session_helpers.go`:**
- `allTranscriptPaths(homeDir, clotildeRoot string, sess *session.Session) ([]string, error)` --
  builds ordered list of transcript paths (previous UUIDs first, then current).
  Uses `claude.TranscriptPath` for computed paths, prefers `metadata.TranscriptPath`
  for the current UUID when available.

Tests in `internal/claude/transcript_test.go` (extend existing):
- `ParseTranscriptStats` extracts token counts from assistant entries
- `ParseTranscriptStats` extracts model IDs (deduplicated)
- `ParseTranscriptStats` counts tool_use blocks by name
- `MergeTranscriptStats` sums tokens and tool uses across transcripts
- `MergeTranscriptStats` unions model lists preserving order
- `MergeTranscriptStats` uses earliest first message and latest last message
- `MergeTranscriptStats` skips nil entries

Tests in `cmd/session_helpers_test.go`:
- `allTranscriptPaths` returns previous + current paths in order
- `allTranscriptPaths` uses `metadata.TranscriptPath` for current UUID when set
- `allTranscriptPaths` computes path when `metadata.TranscriptPath` is empty

### 2. Stats file writer

New file: `internal/claude/stats_file.go`

- `SessionStatsRecord` struct with JSON tags matching the schema above (including `prev_*` fields).
- `StatsDir() (string, error)` -- resolves `$XDG_DATA_HOME/clotilde/stats` or fallback.
- `DailyStatsFilePath(t time.Time) (string, error)` -- full path for the given date.
- `AppendStatsRecord(record SessionStatsRecord) error` -- marshals to JSON, appends to
  daily file. Creates dirs with `os.MkdirAll(dir, 0755)`, opens file with
  `os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)`.
- `FindLastRecord(sessionID string, now time.Time) (*SessionStatsRecord, error)` -- scans
  daily files from today back to 7 days ago, returns the last record matching `sessionID`.
  Returns nil (no error) if no prior record found.
- `ReadStatsFile(path string) ([]SessionStatsRecord, error)` -- reads a daily file,
  returns all records. Skips lines that fail JSON unmarshal (handles truncated writes).
- `ConsolidateStatsFile(path string) ([]SessionStatsRecord, error)` -- reads a daily file,
  deduplicates by `session_id` (keeps last occurrence), writes to a temp file in the same
  directory (`.YYYY-MM-DD.jsonl.tmp`), then atomically renames to the final path. Returns
  the deduplicated records. Only called when the caller detects duplicates.

Tests in `internal/claude/stats_file_test.go`:
- `StatsDir` returns `$XDG_DATA_HOME/clotilde/stats` when env var is set
- `StatsDir` falls back to `~/.local/share/clotilde/stats` when not set
- `DailyStatsFilePath` returns correct `YYYY-MM-DD.jsonl` filename
- `AppendStatsRecord` creates file and parent dirs if missing
- `AppendStatsRecord` appends second record to existing file (two valid JSON lines)
- `AppendStatsRecord` writes valid JSON that round-trips through unmarshal
- `FindLastRecord` returns the latest record for a session from today's file
- `FindLastRecord` falls back to previous days when today has no match
- `FindLastRecord` returns nil when no record found within 7-day window
- `ReadStatsFile` returns all records including duplicates
- `ReadStatsFile` skips lines that fail JSON unmarshal (truncated writes)
- `ConsolidateStatsFile` keeps only the last record per session_id
- `ConsolidateStatsFile` preserves order of first appearance across sessions
- `ConsolidateStatsFile` uses atomic temp-file-then-rename

### 3. Move and refactor session name helpers

Move from `cmd/hook_sessionstart.go` to `cmd/session_helpers.go`:
- `readSessionNameFromEnvFile`
- `findSessionByUUID`

Refactor `getSessionName`: the current function gates env file and UUID fallbacks behind
`source == "compact" || source == "clear"`. SessionEnd also needs the full three-level
fallback. Replace the source-based gate with a `fullFallback bool` parameter:

```go
func resolveSessionName(hookData hookInput, store session.Store, fullFallback bool) (string, error)
```

- `handleStartup`/`handleResume` call with `fullFallback: false` (env var only).
- `handleCompact`/`handleClear` call with `fullFallback: true`.
- `hook_sessionend` calls with `fullFallback: true`.

Same package (`cmd`), no external API change. Existing tests must still pass.

### 4. `hook_sessionend` command

New file: `cmd/hook_sessionend.go`

Structure mirrors `hook_sessionstart.go`:
- Cobra command `sessionend`, hidden, registered under `hookCmd`.
- Reads stdin payload, resolves session name, loads session if possible,
  computes merged stats (including tokens and tool usage), writes to daily JSONL.
- Prints `clotilde: saving session stats...` to stderr on entry.
- Uses double-execution guard (same pattern as SessionStart) to prevent duplicate
  records when both global and per-project hooks are registered.
- Masks SIGINT/SIGTERM during the critical write section.

Tests in `cmd/hook_sessionend_test.go`:
- Happy path: reads payload, resolves session, writes stats record to temp dir
- Falls back to payload `transcript_path` when clotilde root not found
- Exits 0 and logs to stderr when transcript is unreadable
- Exits 0 and logs to stderr when stats file cannot be written
- `session_name` resolved from `CLOTILDE_SESSION_NAME` env var
- `session_name` resolved from env file fallback when env var is unset
- `prev_*` fields are zero on first record for a session
- `prev_*` fields populated from prior record on resumed session
- Record includes token counts and tool usage from transcript
- Record includes model list from transcript
- Double-execution guard prevents duplicate records

### 4b. Crash recovery in SessionStart hook

Modify `cmd/hook_sessionstart.go` (`handleResume`):
- After resolving session name, fast-path check: if `lastAccessed` is within 30 seconds
  of now, skip recovery entirely (double-fire or immediate resume, no crash). This is a
  time comparison only, no I/O.
- If `lastAccessed` is stale, call `FindLastRecord` to check if the previous invocation's
  stats were recorded. `FindLastRecord` short-circuits when a match is found in today's
  file (common case), so the full 7-day scan only happens when no record exists.
- If no record found, compute stats from current transcript and write a recovery record
  with `ended_at` set to the session's `lastAccessed` timestamp.
- Recovery is best-effort: log warning on failure, do not block the resume.

Tests (extend `cmd/hook_test.go`):
- Recovery record written when prior SessionEnd is missing
- Recovery skipped when prior SessionEnd record exists
- Recovery skipped for brand-new sessions (no prior activity)

### 5. Wire into hook config (opt-in)

Modify `internal/claude/hooks.go`:
- Add `HookConfigOptions` struct:
  ```go
  type HookConfigOptions struct {
      StatsEnabled bool
  }
  ```
- Change `GenerateHookConfig(binaryPath string)` to
  `GenerateHookConfig(binaryPath string, opts HookConfigOptions)`.
  When `opts.StatsEnabled` is true, includes `SessionEnd` pointing to
  `clotilde hook sessionend`. When false, omits `SessionEnd`.
- Existing callers pass `HookConfigOptions{}` (zero value = stats disabled).

Modify `cmd/setup.go`:
- Add `--stats` and `--no-stats` flags.
- If neither is passed, prompt interactively: "Track session statistics? [y/N]"
- Store preference in global config (`~/.config/clotilde/config.json`) as
  `"statsTracking": true/false`.
- Pass the preference to `GenerateHookConfig`.

Modify config struct (`internal/config/config.go`):
- Add `StatsTracking bool `json:"statsTracking,omitempty"`` to `Config`. The field is
  only meaningful in the global config; project configs ignore it. Uses `omitempty` so it
  doesn't appear in project config files.

Update `internal/claude/hooks_test.go`:
- Test: `GenerateHookConfig` with stats enabled includes `SessionEnd`.
- Test: `GenerateHookConfig` with stats disabled omits `SessionEnd`.
- Update existing test that asserts `SessionEnd` is empty (now conditional).

---

## Commit Plan

1. `fix(cmd): only write non-empty hook types in mergeHooksIntoSettings` -- prerequisite (done)
2. `feat(claude): extend ParseTranscriptStats with tokens, models, tool usage` -- section 1 (transcript.go)
3. `feat(cmd): add multi-transcript helpers` -- section 1 (session_helpers.go)
4. `feat(claude): add stats file writer for daily session records` -- section 2
5. `refactor(cmd): move session name helpers to session_helpers.go` -- section 3
6. `feat(cmd): add hook sessionend command with signal safety` -- section 4
7. `feat(cmd): add crash recovery for sessions that never ended` -- section 4b
8. `feat(claude): wire SessionEnd into GenerateHookConfig (opt-in)` -- section 5

---

## Resolved Questions

- **SessionEnd payload includes `transcript_path`.** Confirmed via Claude Code docs. The
  payload is: `{"session_id": "...", "transcript_path": "...", "cwd": "...",
  "permission_mode": "...", "hook_event_name": "SessionEnd", "reason": "..."}`. The
  `reason` field indicates why the session ended (exit, sigint, error). The `cwd` field
  could be useful as a fallback for project path resolution.

## Open Questions

- **Worktree edge case:** If the hook fires with a different CWD than where the session
  started, `FindClotildeRoot` fails. The hook falls back to partial stats. The `cwd` field
  from the payload might help here, but it reflects the CWD at session end, not start.
  A proper fix would store the project root in the session env file at start time. Defer
  to follow-up.
- **Stale payload bug ([#9188](https://github.com/anthropics/claude-code/issues/9188)):**
  SessionEnd hooks previously received stale `session_id` and `transcript_path` after `/exit`
  and `--continue`. Reportedly fixed. Verify during smoke testing; if still present, the
  fallback to session metadata (env file / UUID lookup) should cover it.

---

## Landscape: How Other Tools Track Usage

Clotilde stats track **activity** (turns, active time, total time). Existing tools focus on
**cost** (tokens, USD). These are complementary, not overlapping:

| Tool | Focus | Data source | Metrics |
|---|---|---|---|
| **ccusage** ([GitHub](https://github.com/ryoppippi/ccusage)) | Cost tracking | Local JSONL transcripts | Tokens (in/out/cache), cost in USD, per-model breakdown |
| **Claude Code Usage Monitor** ([GitHub](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor)) | Real-time monitoring | Local transcripts | Token burn rate, cost projections, session forecasting |
| **Claude Code Analytics** (built-in) | Org-level productivity | Server-side telemetry | Active users, sessions, lines of code, PRs (Team/Enterprise only) |
| **Claude Code Analytics API** | Programmatic org data | API | Productivity metrics, token/cost data, per-model breakdown |
| **Grafana + OpenTelemetry** | Custom dashboards | OTel export | Token usage, API costs, cache efficiency, session duration |
| **Clotilde stats** (this feature) | Per-session activity + usage | Local transcripts + hook | Turns, active/total time, tokens, models, tool usage, per-project, named sessions |

Clotilde's differentiator: named sessions with human-friendly identifiers, per-project
grouping, and a combined activity + usage view (turns, time, tokens, tools in one record).
Unlike ccusage (which focuses on cost in USD), clotilde tracks the interaction shape: how
many turns, which tools, how long, alongside raw token counts. Unlike the built-in
analytics (Team/Enterprise only, server-side), clotilde works locally for individual users.

---

## Scrutiny Log

16 issues identified across two review passes. All resolved inline in the design above.

| # | Issue | Resolution |
|---|---|---|
| 1 | Double-execution guard needed | Added to section 4 (same pattern as SessionStart) |
| 2 | `getSessionName` source gating | Refactored to `resolveSessionName(hookData, store, fullFallback)` in section 3 |
| 3 | `allTranscriptPaths` missing `homeDir` | Fixed signature in section 1 |
| 4 | `ConsolidateStatsFile` rewrite safety | Atomic temp-file-then-rename in same directory (section 2) |
| 5 | Write-on-read concern | Consolidation only runs when caller detects duplicates (dedup section) |
| 6 | `/fork` section misplaced | Moved to design section |
| 7 | Hooks merge bug prerequisite | Added as commit 1 in commit plan |
| 8 | `mergeTranscriptStats` package location | `MergeTranscriptStats` in `internal/claude/transcript.go` (section 1) |
| 9 | `signal.Ignore` scope too broad | Scoped to file write only, no defer (signal safety section) |
| 10 | `Config` struct lacks global-only fields | Add `StatsTracking` with `omitempty` to shared `Config` (section 5) |
| 11 | Nil map panic on `ToolUses` | `make(map[string]int)` in init (transcript parsing section) |
| 12 | Content array parsing cost | `json.RawMessage` + conditional unmarshal for assistant only (transcript parsing section) |
| 13 | `GenerateHookConfig` parameter scaling | `HookConfigOptions` struct (section 5) |
| 14 | Crash recovery latency on every resume | Fast-path time check before I/O (section 4b) |
| 15 | `ConsolidateStatsFile` temp file location | Same directory as target for atomic rename (section 2) |
| 16 | File permissions unspecified | 0755 dirs, 0644 files (section 2) |
