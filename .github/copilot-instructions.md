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
  export/               -> Session transcript export to self-contained HTML
  outputstyle/          -> Output style management
  ui/                   -> TUI components (dashboard, picker, table, confirm)
  util/                 -> UUID generation, filesystem helpers
  testutil/             -> Test utilities (fake claude binary)
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

For efficient tail reads (last model, last timestamp), use the seek-to-tail
pattern established in `internal/claude/transcript.go` (`ExtractLastModel`,
`LastTranscriptTime`).

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

## Conventions

- Go module: `github.com/fgrehm/clotilde`
- Test framework: Ginkgo/Gomega
- Linting: golangci-lint v2, formatting: gofumpt (via `golangci-lint fmt`)
- Commit format: Conventional Commits, present tense, under 72 chars
- `CHANGELOG.md` uses Keep a Changelog format
