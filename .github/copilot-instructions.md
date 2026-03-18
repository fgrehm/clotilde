# Copilot Instructions for clotilde

Clotilde is a Go CLI wrapper around Claude Code that adds named session
management. It wraps Claude Code session UUIDs with human-friendly names,
enabling easy switching between multiple parallel conversations.

## Architecture

```
cmd/                    -> Cobra command implementations
internal/
  session/              -> Session data structures, storage (FileStore), validation
  config/               -> Config management, path resolution
  claude/               -> Claude CLI invocation, path conversion, hook generation
                           Includes InvokeStreaming for non-interactive calls
  export/               -> Session transcript export to self-contained HTML
  outputstyle/          -> Output style management
  ui/                   -> TUI components (dashboard, picker, table, confirm)
  util/                 -> UUID generation, filesystem helpers
  testutil/             -> Test utilities (fake claude binary)
  tour/                 -> Interactive codebase tours (CodeTour format)
                           Tour loading, generation, validation; prompt construction
  server/               -> HTTP server for tour browser UI, WebSocket chat, REST API
```

All packages are under `internal/`; this is a binary, not a library.

## Key Design Decisions

- Thin, non-invasive wrapper. Never modifies Claude Code itself.
- Session data stored in `.claude/clotilde/sessions/<name>/metadata.json`.
- Invokes `claude` CLI with mapped UUIDs via `--session-id` / `--resume`.
- Global hooks in `~/.claude/settings.json` (installed by `clotilde setup`).
- Lazy directory creation: session-creating commands auto-create
  `.claude/clotilde/sessions/` on first use.
- Session-reading commands return friendly messages when no sessions exist.
- Double-hook execution guard via `CLOTILDE_HOOK_EXECUTED` env var prevents
  duplicate output when both global and per-project hooks exist.

## Interactive Codebase Tours (Experimental)

Tours are browser-based walkthroughs that combine code viewing with Claude chat.
Key patterns:

- **Non-interactive Claude invocation** (`InvokeStreaming`): Spawns `claude` CLI
  with `--output-format stream-json --verbose`, captures line-delimited JSON on
  stdout, calls callback for each line. No TTY needed.

- **WebSocket chat**: Browser sends message + context (tour, step, file, line) to
  `/ws/chat`. Server builds prompt with context, invokes claude via InvokeStreaming,
  streams tokens back as JSON over WebSocket, renders as markdown with syntax highlighting.

- **Persistent tour session**: `tour serve` creates `tour-<repo-name>` Clotilde
  session with system prompt replacement (full tour guide role). Chat uses `--resume`
  for multi-turn continuity, same approach as regular sessions.

- **Tour generation**: `tour generate` builds a prompt telling Claude to crawl the
  repo autonomously using its own file tools (Glob, Read, Grep), invokes via
  InvokeStreaming with `--permission-mode bypassPermissions`, streams tool progress
  to stderr, validates JSON output, saves to `.tours/<name>.tour`. Invalid output
  saved to `.tour.invalid` for debugging. `--focus` narrows the topic.

## Session Hooks

A single `SessionStart` hook (`clotilde hook sessionstart`) handles all
lifecycle events (startup, resume, compact, clear) based on the `source`
field in JSON input from Claude Code. Fork registration, session ID updates,
and context injection all happen through this hook.

## Session Transcript Paths

Transcripts live in `~/.claude/projects/<encoded-project-dir>/<uuid>.jsonl`.
When a user runs `/clear`, Claude Code creates a new UUID; the old one is
appended to `previousSessionIds` in `metadata.json`. Commands that need the
full conversation history (stats, export) must collect all paths via the
shared helper:

```go
paths := allTranscriptPaths(sess, clotildeRoot, homeDir) // cmd/session_helpers.go
```

For efficient tail reads (last model, last timestamp), use `forEachTailLine`
in `internal/claude/transcript.go`. It handles tail-seeking, newline boundary
detection, and uses `bufio.Reader` with `ReadSlice` + drain so oversized
lines are skipped without halting (unlike `bufio.Scanner` which stops
permanently on `ErrTooLong`). All tail readers (`ExtractLastModel`,
`LastTranscriptTime`, `ExtractModelAndLastTime`) use this helper.

`ParseTranscriptStats` uses the same `ReadSlice` + drain approach with a 1MB
buffer (reads the full file, not just the tail).

Multi-transcript loops (stats, export) skip `os.IsNotExist` errors (expected
for old `/clear` transcripts) and surface all other errors to the user.

## Export HTML Format

`export.BuildHTML` base64-encodes all session entries into a
`<script id="session-data" type="application/json">` tag. Test assertions
about message content must decode this data first — a plain
`ContainSubstring` on the raw HTML will always fail:

```go
const marker = `<script id="session-data" type="application/json">`
start := strings.Index(html, marker) + len(marker)
end := strings.Index(html[start:], "</script>")
decoded, _ := base64.StdEncoding.DecodeString(html[start : start+end])
Expect(string(decoded)).To(ContainSubstring("expected text"))
```

## Injectable Function Variables

Package-level `var` functions allow test overrides without interface
abstraction. Always restore with `t.Cleanup`:

```go
// e.g. internal/util/names.go
var GitBranchFunc = defaultGitBranch

// in tests
orig := util.GitBranchFunc
util.GitBranchFunc = func() string { return "feature/test" }
t.Cleanup(func() { util.GitBranchFunc = orig })
```

Other examples: `claude.ClaudeBinaryPathFunc`, `claude.VerboseFunc`.

## Testing Patterns

- Tests that create transcript files set `HOME` to a temp dir to avoid
  polluting the real `~/.claude/projects/` directory.
- `t.Run` subtests use descriptive fallback names for empty inputs
  (e.g. `"(empty)"`) to avoid panics.
- `export.FilterTranscript` uses `ReadBytes('\n')` intentionally; export
  needs full line content for HTML output, so oversized lines cannot be
  skipped. This is a known tradeoff, not a bug.

## Conventions

- Go module: `github.com/fgrehm/clotilde`
- Test framework: Ginkgo/Gomega for cmd/internal packages, stdlib
  `testing` for unit tests in util/claude
- Linting: golangci-lint v2, formatting: gofumpt (via `golangci-lint fmt`)
- Commit format: Conventional Commits, present tense, under 72 chars
- `CHANGELOG.md` uses Keep a Changelog format
