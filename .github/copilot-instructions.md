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
- Path walk-up stops at `$HOME` to avoid treating `~/.claude/` (Claude
  Code's global config) as a project marker.
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

## Tooling

- Go version: see `go.mod`.
- Linter: golangci-lint v2, managed as a Go tool dependency. Run `make lint` or
  `go tool golangci-lint run ./...`. Config in `.golangci.yml`.
- Formatting: `make fmt` runs gofumpt + goimports via `go tool golangci-lint fmt`.
- Dead code: `make deadcode` runs `go tool deadcode ./...` (hard gate in CI).
- Vulnerability check: `make govulncheck` runs `go tool govulncheck ./...` (hard gate in CI).
- Complexity: `make audit` runs gocyclo (informational at 15, hard gate at 30 in CI).
- Tests: `make test` runs Ginkgo with `--randomize-all --race`.
- Pre-commit hook: `.githooks/pre-commit` auto-formats and lints staged files.
  Run `make setup-hooks` to activate.
- Release: tag-triggered via GoReleaser. Release notes extracted from `CHANGELOG.md`.
  See the Releasing section in CLAUDE.md.

## CHANGELOG

When reviewing PRs, verify that `CHANGELOG.md` has an `[Unreleased]` entry for any
user-facing change (features, fixes, breaking changes). Use
[Keep a Changelog](https://keepachangelog.com/) format.
