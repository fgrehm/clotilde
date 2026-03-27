# Clotilde

A power-user companion for Claude Code.

## Why?

Claude Code has gotten better at session management: `-n` names sessions, `/branch` creates forks, and `/resume` shows a picker. But daily use still has friction:

- **Flags don't stick**: Want opus for deep work and haiku for quick tasks? You re-pass `--model` every time. Clotilde persists model, effort level, permissions, and output style in each session — automatically re-applied on every resume.
- **No reusable presets**: Clotilde profiles let you define named configurations (model, permissions, output style) in a config file and apply them with `--profile <name>`.
- **No persistent context**: Clotilde's `--context` flag attaches a note to a session (ticket number, current goal) that's injected into Claude automatically at startup and resume.
- **Every session persists**: Clotilde's incognito sessions auto-delete all data — metadata, transcripts, logs — when you exit.
- **Fork only from inside Claude**: `clotilde fork` creates a branched conversation from anywhere on the command line, with parent/child tracking.
- **Common flag combos are a mouthful**: `--fast` means haiku + low effort. `--yolo` means bypass permissions. One flag instead of two or three.
- **No completion for session names**: Clotilde adds shell completion for session names and profile names across bash, zsh, and fish.
- **Transcripts are unreadable JSONL**: `clotilde export` renders a session as self-contained HTML with syntax highlighting, collapsible thinking blocks, and formatted tool outputs.

## What Clotilde does

```bash
# Start and resume named sessions
clotilde start auth-feature
clotilde resume auth-feature

# Settings stick — set them once, re-applied automatically on every resume
clotilde start deep-work --model opus --effort high
clotilde resume deep-work                             # opus + high effort, no flags needed

# Profiles for reusable configurations
clotilde start spike --profile quick                 # haiku + bypass permissions
clotilde start review --profile strict               # deny Bash/Write, ask mode

# Session context, injected into Claude at startup
clotilde start auth-feature --context "working on GH-123"

# Incognito: auto-deletes everything on exit
clotilde incognito

# Fork a session from the command line
clotilde fork auth-feature auth-experiment

# Export a session as self-contained HTML
clotilde export auth-feature

# Session stats: turns, tokens, tool usage
clotilde stats auth-feature
clotilde stats --all                                  # aggregate across recent sessions

# Interactive codebase tours with Claude chat (experimental)
clotilde tour generate --focus "authentication"
clotilde tour serve
```

Clotilde is a thin wrapper: it maps human-readable names to Claude Code UUIDs, invokes `claude` with the right flags, and never patches or modifies Claude Code itself.

## Installation

**Download binary** (recommended)

```bash
# Linux (amd64)
curl -fsSL https://github.com/fgrehm/clotilde/releases/latest/download/clotilde_linux_amd64.tar.gz | tar xz -C ~/.local/bin

# Linux (arm64)
curl -fsSL https://github.com/fgrehm/clotilde/releases/latest/download/clotilde_linux_arm64.tar.gz | tar xz -C ~/.local/bin

# macOS (Apple Silicon)
curl -fsSL https://github.com/fgrehm/clotilde/releases/latest/download/clotilde_darwin_arm64.tar.gz | tar xz -C ~/.local/bin

# macOS (Intel)
curl -fsSL https://github.com/fgrehm/clotilde/releases/latest/download/clotilde_darwin_amd64.tar.gz | tar xz -C ~/.local/bin
```

**mise:**
```bash
mise use github:fgrehm/clotilde
```

**Go install:**
```bash
go install github.com/fgrehm/clotilde@latest
```

**Build from source:**
```bash
git clone https://github.com/fgrehm/clotilde
cd clotilde
make build
make install  # installs to ~/.local/bin
```

## Quick Start

```bash
# One-time setup (registers SessionStart hooks globally)
clotilde setup

# Start a new named session
clotilde start auth-feature

# Resume it later
clotilde resume auth-feature

# List all sessions
clotilde list

# Inspect a session (settings, context, files)
clotilde inspect auth-feature

# Fork a session to try something different
clotilde fork auth-feature experiment

# Delete when done
clotilde delete experiment
```

## How It Works

Clotilde never patches or modifies Claude Code.

- Each session is a folder in `.claude/clotilde/sessions/<name>/` containing metadata and optional settings
- `clotilde setup` registers a SessionStart hook in `~/.claude/settings.json` that handles context injection and `/clear` UUID tracking
- Claude Code is invoked with `--session-id` (new sessions), `--resume` (existing), and `--settings` (model, effort, permissions)

**Worktrees:** `.claude/clotilde/` lives in each worktree's `.claude/` directory, so each worktree gets its own independent sessions. Use worktrees for major branches, Clotilde for managing multiple conversations within each.

**Gitignore:** `.claude/clotilde/` contains ephemeral, per-user session state — add it to your `.gitignore`.

## Features

### Sticky Session Settings

Flags set on `start` and `incognito` are saved to the session's `settings.json` and re-applied automatically on every resume. No need to repeat flags.

```bash
# Set once
clotilde start deep-work --model opus --effort high

# Resume any number of times — settings apply automatically
clotilde resume deep-work
```

**What gets persisted:** `--model`, `--effort`, `--fast` (stores `model=haiku` + `effortLevel=low`), `--permission-mode`, `--allowed-tools`, `--disallowed-tools`, `--add-dir`, `--output-style`.

Override for a specific resume by passing the flag — CLI always wins over stored settings:

```bash
clotilde resume deep-work --model sonnet   # one-off override, stored settings unchanged
```

### Session Context

Attach a note to a session so Claude knows what you're working on. Stored in session metadata and injected automatically at every startup:

```bash
clotilde start auth-feature --context "working on ticket GH-123"
clotilde fork auth-feature experiment --context "trying JWT instead of sessions"

# Update when switching tasks
clotilde resume auth-feature --context "now on GH-456"
```

Forked sessions inherit context from the parent unless overridden. `clotilde inspect <name>` shows the stored context.

### Incognito Sessions

Incognito sessions auto-delete themselves — metadata, transcripts, and agent logs — when you exit:

```bash
clotilde incognito
clotilde incognito quick-test --fast
```

Cleanup runs on normal exit (Ctrl+D, `/exit`). If the process is killed (SIGKILL), the session may persist; use `clotilde delete <name>` to clean up manually.

You cannot fork *from* an incognito session, but you can fork *to* one: `clotilde fork auth-feature temp --incognito`.

### Forking

Fork creates a new session starting from the parent's conversation history:

```bash
clotilde fork auth-feature experiment
clotilde fork auth-feature --incognito                          # random name, auto-deletes
clotilde fork auth-feature temp --context "trying a different approach"
```

Settings and context are inherited from the parent. The fork gets its own UUID and metadata; the parent is unaffected.

### Profiles

Define named presets in a config file and apply them with `--profile`:

```json
{
  "profiles": {
    "quick": {
      "model": "haiku",
      "permissionMode": "bypassPermissions"
    },
    "strict": {
      "permissions": {
        "deny": ["Bash", "Write"],
        "defaultMode": "ask"
      }
    },
    "research": {
      "model": "sonnet",
      "outputStyle": "Explanatory"
    }
  }
}
```

Place profiles in `~/.config/clotilde/config.json` (global, respects `$XDG_CONFIG_HOME`) or `.claude/clotilde/config.json` (project-scoped). Project profiles override global ones with the same name.

```bash
clotilde start spike --profile quick
clotilde start sandboxed --profile strict

# CLI flags override profile values
clotilde start research --profile quick --model sonnet
```

**Profile fields:** `model`, `permissionMode`, `permissions` (allow/deny/ask/additionalDirectories/defaultMode/disableBypassPermissionsMode), `outputStyle`.

**Precedence:** global profile → project profile → CLI flags.

### Shorthand Flags

Available on all commands (`start`, `incognito`, `resume`, `fork`):

```bash
# Permission modes
clotilde start refactor --accept-edits    # auto-approve edits
clotilde incognito --yolo                 # bypass all permission checks
clotilde start spike --plan              # plan mode
clotilde resume my-session --dont-ask    # approve everything without asking

# Fast mode: haiku + low effort (persisted in session settings on start/incognito)
clotilde start quick-check --fast
clotilde incognito --fast --yolo
```

`--fast` cannot be combined with `--model` or `--effort`. Permission shortcuts are mutually exclusive with each other and with `--permission-mode`.

### Output Styles

Customize how Claude communicates in a session:

```bash
# Built-in styles (case-sensitive)
clotilde start myfeature --output-style Explanatory
clotilde start myfeature --output-style Learning

# Custom style from file
clotilde start myfeature --output-style-file ./my-style.md

# Inline custom content (creates a session-specific style file)
clotilde start myfeature --output-style "Be concise and use bullet points"

# Existing named style (from .claude/output-styles/ or ~/.claude/output-styles/)
clotilde start myfeature --output-style my-project-style
```

Session-specific custom styles are stored in `.claude/output-styles/clotilde/<session-name>.md` and should be gitignored. Team-shared styles go in `.claude/output-styles/` (committed to git).

### Pass-Through Flags

Pass any Claude Code flag directly using `--`:

```bash
clotilde start my-session -- --debug api,hooks
clotilde resume my-session -- --verbose
```

Pass-through flags apply to that invocation only and are not persisted. Use named flags (`--model`, `--effort`, etc.) if you want settings to stick across resumes.

### Interactive Tours (Experimental)

Browser-based codebase walkthroughs with an integrated Claude chat sidebar. Tours are CodeTour JSON files (`.tours/*.tour`) that step through key files with descriptions and line references.

```bash
# Generate a tour using Claude
clotilde tour generate
clotilde tour generate --focus "authentication" --name auth-flow --model sonnet

# List available tours
clotilde tour list

# Start the local tour server (default: http://localhost:3333)
clotilde tour serve
clotilde tour serve --model sonnet --port 8080
```

The tour server creates a persistent `tour-<repo-name>` Clotilde session for chat continuity. Chat context includes the current tour step, file, and line. APIs may change — feedback welcome.

## Commands

### `clotilde setup [--local] [--stats] [--no-stats]`

One-time setup. Registers a SessionStart hook in `~/.claude/settings.json`.

```bash
clotilde setup              # registers hooks globally (recommended)
clotilde setup --local      # registers in ~/.claude/settings.local.json instead
clotilde setup --stats      # also registers a SessionEnd hook for stats tracking
clotilde setup --no-stats   # disable stats tracking
```

After setup, `clotilde start` works in any project directory.

### `clotilde start [name] [options]`

Start a new named session. Auto-generates a name like `2026-03-09-happy-fox` if none is provided.

```bash
clotilde start
clotilde start my-session
clotilde start bugfix --model haiku --effort low
clotilde start spike --profile quick
clotilde start sandboxed --permission-mode plan --allowed-tools "Read,Bash(npm:*)" --disallowed-tools "Write"
clotilde start auth-feature --context "working on GH-123"
```

**Options:**
- `--model <model>` — Model (haiku, sonnet, opus). Persisted in session settings.
- `--effort <level>` — Reasoning effort (low, medium, high, max). Persisted in session settings.
- `--fast` — haiku + low effort. Persisted in session settings.
- `--profile <name>` — Named profile (baseline; CLI flags override).
- `--context <text>` — Session context, injected at startup.
- `--incognito` — Auto-delete session on exit.
- `--accept-edits` — Shorthand for `--permission-mode acceptEdits`.
- `--yolo` — Shorthand for `--permission-mode bypassPermissions`.
- `--plan` — Shorthand for `--permission-mode plan`.
- `--dont-ask` — Shorthand for `--permission-mode dontAsk`.
- `--permission-mode <mode>` — acceptEdits, bypassPermissions, default, dontAsk, plan. Persisted.
- `--allowed-tools <tools>` — Comma-separated allowed tools (e.g. `Bash(npm:*),Read`). Persisted.
- `--disallowed-tools <tools>` — Comma-separated denied tools. Persisted.
- `--add-dir <directories>` — Additional directories to allow access to. Persisted.
- `--output-style <style>` — Built-in name, existing style name, or inline content. Persisted.
- `--output-style-file <path>` — Path to custom output style file. Persisted.
- `--append-system-prompt <text>` — System prompt text to append to Claude's default.
- `--append-system-prompt-file <path>` — System prompt file to append.
- `--replace-system-prompt <text>` — Replace Claude's default system prompt entirely.
- `--replace-system-prompt-file <path>` — Replace default system prompt from file.

### `clotilde incognito [name] [options]`

Start an incognito session that auto-deletes on exit. Same options as `clotilde start` (`--incognito` is implicit).

```bash
clotilde incognito
clotilde incognito quick-test
clotilde incognito --fast --yolo
```

### `clotilde resume [name] [options]`

Resume a session by name. Shows an interactive picker if no name is provided (TTY only). Stored settings from `settings.json` are applied automatically; flags override them for this invocation only.

```bash
clotilde resume auth-feature
clotilde resume auth-feature --model sonnet        # one-off model override
clotilde resume auth-feature --fast                # one-off fast mode
clotilde resume auth-feature --accept-edits
clotilde resume auth-feature --context "now on GH-456"
```

**Options:**
- `--context <text>` — Update the stored session context.
- `--model <model>` — Override model for this invocation only.
- `--effort <level>` — Override effort level for this invocation only.
- `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast` — Shorthand flags.

### `clotilde fork <parent> [name] [options]`

Fork a session. Inherits settings and context from the parent. If no name is provided with `--incognito`, a random name is generated.

```bash
clotilde fork auth-feature auth-experiment
clotilde fork auth-feature --incognito
clotilde fork auth-feature temp --context "trying different approach"
```

**Options:**
- `--context <text>` — Context for the fork (inherits from parent if not specified).
- `--incognito` — Fork as incognito session.
- `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast` — Shorthand flags.

**Note:** Cannot fork *from* incognito sessions; can fork *to* them.

### `clotilde list`

List all sessions with name, model, and last used timestamp.

### `clotilde inspect <name>`

Show detailed session info: UUID, timestamps, settings, context, associated files, and Claude Code data status.

### `clotilde delete <name> [--force]`

Delete a session and all associated Claude Code data (current and previous transcripts, agent logs).

- `--force, -f` — Skip confirmation.

### `clotilde stats [name] [--all]`

Show session statistics: turns, timing, tokens, models used, tool usage. Parsed from Claude Code transcripts.

```bash
clotilde stats auth-feature
clotilde stats --all         # aggregate across sessions active in last 7 days
```

`--all` reads from daily JSONL stats files (enable with `clotilde setup --stats`), falling back to parsing transcripts. Stats files at `$XDG_DATA_HOME/clotilde/stats/` are available for external tools and dashboards.

### `clotilde stats backfill`

Generate stats records from existing transcripts for sessions that don't have them yet.

### `clotilde export <name> [options]`

Export a session as self-contained HTML with syntax-highlighted code, collapsible thinking blocks, and expandable tool outputs.

```bash
clotilde export auth-feature
clotilde export auth-feature -o ~/Desktop/auth-session.html
clotilde export auth-feature --stdout | wc -c
```

**Options:**
- `-o, --output <path>` — Output path (default: `./<name>.html`).
- `--stdout` — Write to stdout.

**Keyboard shortcuts** in the exported HTML: `Ctrl+T` toggles thinking blocks, `Ctrl+O` toggles tool outputs.

### `clotilde tour list [--dir PATH]`

List `.tour` files in the project's `.tours/` directory. `--dir` sets the repository root (default: current directory).

### `clotilde tour serve [options]`

Start the interactive tour server at `http://localhost:3333`.

**Options:** `--dir PATH`, `--port PORT` (default: 3333), `--model MODEL` (default: haiku).

### `clotilde tour generate [options]`

Generate a tour by having Claude analyze the codebase. Saved to `.tours/<name>.tour`. On failure, raw output goes to `.tours/<name>.tour.invalid`.

**Options:** `--dir PATH`, `--name NAME` (default: overview), `--focus FOCUS`, `--model MODEL` (default: sonnet).

### `clotilde` (no subcommand)

Interactive dashboard in TTY: start a new session, resume, fork, list, or delete.

### `clotilde completion <shell>`

Generate shell completion scripts for bash, zsh, fish, or powershell. See `clotilde completion --help` for setup instructions.

## Related Work

Claude Code now has native session naming (`-n`/`--name`), `/rename`, `/branch`, and a `/resume` picker. Clotilde uses these under the hood and focuses on what Claude Code doesn't provide: sticky settings, profiles, context injection, incognito sessions, forking by name, session export, and shorthand flags.

**On `/branch` and `/rename`:** Clotilde doesn't detect or track when you use these inside Claude. For clotilde-managed forks with parent/child tracking, use `clotilde fork`. Sessions created via `/branch` live outside Clotilde's tracking.

### Other tools in this space

- [**tweakcc**](https://github.com/Piebald-AI/tweakcc) — Patches Claude Code to add custom system prompts, toolsets, themes, and more
- [**claude-code-transcripts**](https://github.com/simonw/claude-code-transcripts) — Python tool for converting JSONL transcripts to HTML (Simon Willison)
- [**claude-code-log**](https://github.com/daaain/claude-code-log) — Python CLI for transcript viewing with TUI browser
- [**OpCode**](https://github.com/winfunc/opcode) — GUI app for managing Claude Code sessions, agents, and background tasks

Clotilde differs in being non-invasive (no patching), a single Go binary with no runtime dependencies, and focused on daily ergonomics rather than UI.

## Development

**Requirements:** Go 1.25+, Make

```bash
make build         # build to dist/clotilde
make test          # run tests
make test-watch    # tests in watch mode
make coverage      # coverage report
make fmt           # format code
make lint          # run linter
make deadcode      # check for unreachable functions
make install       # install to ~/.local/bin
```

**Git hooks** (recommended — runs format and lint on commit):
```bash
make setup-hooks
```

`make lint` requires golangci-lint v2.x to match CI:
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

## About the Name

A traditional Brazilian name — sometimes considered old-fashioned or humorous — which adds a light, unpretentious personality to the tool. Pronounced more or less like **KLOH-teel-dee** (Portuguese-ish).

---

Built with [Claude Code](https://claude.ai/code).
