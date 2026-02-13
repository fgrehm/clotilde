# Clotilde

A CLI for managing named Claude Code sessions, with support for forking.

## About the Name

A traditional Brazilian name - sometimes considered old-fashioned or humorous - which adds a light, unpretentious personality to the tool. Pronounced more or less like **KLOH-teel-dee** (Portuguese-ish).

## Why?

Claude Code's `--continue` and `--resume` work great for linear workflows, but managing multiple parallel conversations gets messy:

- **`--resume` picker limitations**: Fine for 2-3 sessions, but scales poorly. The picker often shows unhelpful text like "This session is being continued from a previous conversation that ran out of context. The conversation is summarized below:..." Which one was your auth work again?
- **Session identity**: UUIDs and auto-generated summaries don't convey the purpose or context of each conversation at a glance.

## What Clotilde does

You often need multiple conversations, even within a single branch:
- Main feature work
- Quick bug investigation
- Experimental refactoring (forked from main work to explore tangents)
- Different prompts for different modes (review vs. implementation)

Clotilde gives sessions memorable names so you can easily switch contexts and resume previous conversations.

```bash
# Jump between sessions by name
clotilde resume auth-feature
clotilde resume bugfix-db-timeout

# Fork a session to explore a tangent
clotilde fork auth-feature auth-alternative-approach
```

## How It Works

Clotilde wraps Claude Code session UUIDs with human-friendly names:

- Sessions stored as folders in `.claude/clotilde/sessions/<name>/`
- Each session has metadata, optional settings, system prompt, and context files
- Global context file (`.claude/clotilde/context.md`) - What you're working on (ticket info, task goal)
- SessionStart hooks automatically register forked sessions and load context
- Works alongside Claude Code without patching or modifying it

**Note on worktrees:** Since `.claude/clotilde/` lives in each worktree's `.claude/` directory, each worktree gets its own independent sessions and global context. This makes Clotilde and worktrees complementary - use worktrees for major branches/features, use Clotilde for managing multiple conversations within each worktree.


## Installation

**Download binary** (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/fgrehm/clotilde/releases).

**Go install:**
```bash
go install github.com/fgrehm/clotilde@latest
```

**Build from source:**
```bash
git clone https://github.com/fgrehm/clotilde
cd clotilde
make build
make install  # Installs to ~/.local/bin
```

## Quick Start

```bash
# Initialize in your project
clotilde init

# Start a new named session
clotilde start auth-feature

# Resume it later
clotilde resume auth-feature

# List all sessions
clotilde list

# Inspect a session
clotilde inspect auth-feature

# Fork a session to try something different
clotilde fork auth-feature experiment

# Delete when done
clotilde delete experiment
```

## Incognito Sessions

Incognito sessions automatically delete themselves when you exit, useful for:
- Quick one-off queries
- Testing/experimentation
- Sensitive work that shouldn't persist
- Keeping session list clean

### Usage

Create incognito session:
```bash
clotilde incognito quick-test
# or
clotilde start --incognito quick-test
```

Fork into incognito session:
```bash
clotilde fork my-session temp-fork --incognito
```

**Note:** Incognito sessions are deleted on normal exit (Ctrl+D, `/exit`). If the process crashes or is killed (SIGKILL), the session may persist. Use `clotilde delete <name>` to clean up manually if needed.

**Limitations:**
- Cannot fork FROM incognito sessions (they'll auto-delete when you exit)
- CAN fork TO incognito sessions (create an incognito fork of a regular session)

## Shorthand Flags

Common permission modes and presets have short, memorable flags available on all session commands (`start`, `incognito`, `resume`, `fork`):

```bash
# Permission mode shortcuts
clotilde start refactor --accept-edits    # auto-approve edits, ask for the rest
clotilde incognito --yolo                 # bypass all permission checks
clotilde start spike --plan               # plan mode
clotilde resume my-session --dont-ask     # approve everything

# Fast mode (haiku + low effort)
clotilde start quick-check --fast
clotilde incognito --fast --yolo          # quick throwaway, no prompts

# Works on resume and fork too
clotilde resume my-session --fast
clotilde fork my-session experiment --accept-edits
```

**Permission shortcuts** (`--accept-edits`, `--yolo`, `--plan`, `--dont-ask`) are mutually exclusive with each other and with `--permission-mode`.

**`--fast`** sets `--model haiku` and `--effort low`. Cannot be combined with `--model`.

## Pass-Through Flags

Pass additional Claude Code flags directly using `--` separator:

```bash
# Debug mode
clotilde start my-session -- --debug api,hooks

# Verbose output
clotilde resume my-session -- --verbose

# Multiple flags
clotilde incognito test -- --debug --permission-mode plan
```

**Common use cases:**
- `--debug [filter]` - Debug specific categories (api, hooks, mcp, etc.)
- `--verbose` - Verbose output
- `--permission-mode <mode>` - Override permission mode (acceptEdits, plan, etc.)
- `--dangerously-skip-permissions` - Bypass permissions (for sandboxed environments)
- `--print` - Non-interactive output for scripting
- `--output-format json` - JSON output

**Note:** These flags are passed directly to Claude Code for that session only. To persist settings across sessions, use the `--model` flag or configure `settings.json`.

## Commands

### `clotilde init [--global]`

Initialize clotilde in the current project. Creates `.claude/clotilde/` directory and configures hooks.

By default, hooks are installed in `.claude/settings.local.json` (local to your machine, not committed to git). Use `--global` to install hooks in `.claude/settings.json` instead (shared with team, committed to git).

```bash
# Initialize with local hooks (default - recommended for experimental use)
clotilde init

# Initialize with project-wide hooks (team shares clotilde setup)
clotilde init --global
```

**Why settings.local.json by default?**
- Clotilde is experimental - people can try it without affecting team members
- `.local.json` files are typically gitignored, keeping your config private
- Team members who don't use clotilde won't see the hooks in project settings

**Note:** The `.claude/clotilde/` directory (containing session metadata, transcripts paths, and context) should be gitignored. This is intentional - sessions are ephemeral, per-user state that shouldn't be committed to the repository. Each developer maintains their own independent session list.

### `clotilde start <name> [options]`

Start a new named session.

```bash
# Basic usage
clotilde start my-session

# With custom model
clotilde start bugfix --model haiku

# With custom system prompt (append to default)
clotilde start refactoring --append-system-prompt-file prompts/architect.md
clotilde start review --append-system-prompt "Be critical and thorough"

# Replace Claude's default system prompt entirely
clotilde start minimal --replace-system-prompt-file prompts/custom.md
clotilde start focused --replace-system-prompt "You are a focused coding assistant"

# Create incognito session
clotilde start quick-test --incognito

# With permissions
clotilde start sandboxed --permission-mode plan \
  --allowed-tools "Read,Bash(npm:*)" \
  --disallowed-tools "Write" \
  --add-dir "../docs"

# Combine options
clotilde start research --model haiku --append-system-prompt "Focus on exploration"
```

**Options:**
- `--model <model>` - Model to use (haiku, sonnet, opus), defaults to whatever is specified on your project configs (`.claude/settings.json`) or globally (`~/.claude/settings.json`)
- `--fast` - Use haiku model with low effort for quick tasks
- `--append-system-prompt <text>` - Add system prompt text (appends to Claude's default)
- `--append-system-prompt-file <path>` - Add system prompt from file (appends to Claude's default)
- `--replace-system-prompt <text>` - Replace Claude's default system prompt entirely with custom text
- `--replace-system-prompt-file <path>` - Replace Claude's default system prompt entirely with file contents
- `--incognito` - Create incognito session (auto-deletes on exit)
- `--accept-edits` - Shorthand for `--permission-mode acceptEdits`
- `--yolo` - Shorthand for `--permission-mode bypassPermissions`
- `--plan` - Shorthand for `--permission-mode plan`
- `--dont-ask` - Shorthand for `--permission-mode dontAsk`
- `--permission-mode <mode>` - Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)
- `--allowed-tools <tools>` - Comma-separated list of allowed tools (e.g. `Bash(npm:*),Read`)
- `--disallowed-tools <tools>` - Comma-separated list of disallowed tools (e.g. `Write,Bash(git:*)`)
- `--add-dir <directories>` - Additional directories to allow tool access to
- `--output-style <style>` - Output style: `default`, `Explanatory`, `Learning`, existing style name, or custom content
- `--output-style-file <path>` - Path to custom output style file

### Output Styles

Customize how Claude communicates in each session using built-in or custom output styles:

```bash
# Built-in styles
clotilde start myfeature --output-style default
clotilde start myfeature --output-style Explanatory
clotilde start myfeature --output-style Learning

# Existing project/user styles (by name)
clotilde start myfeature --output-style my-project-style
clotilde start myfeature --output-style my-personal-style

# Custom inline content (creates new session-specific style)
clotilde start myfeature --output-style "Be concise and use bullet points"

# Custom from file (creates new session-specific style)
clotilde start myfeature --output-style-file ./my-style.md
```

**How `--output-style` flag works:**
1. Checks if value matches existing style in `.claude/output-styles/` or `~/.claude/output-styles/`
2. If found, uses that style (no new file created)
3. Else checks if value is a built-in style (`default`, `Explanatory`, `Learning`)
4. Otherwise treats value as inline content and creates `.claude/output-styles/clotilde/<session-name>.md`

**Note:** Case sensitivity matters! `"default"` is lowercase, `"Explanatory"` and `"Learning"` are capitalized.

**Storage:** Session-specific custom styles are stored in `.claude/output-styles/clotilde/<session-name>.md` and should be gitignored (much like `.claude/clotilde`, they're ephemeral per-user customizations). Team members can share output styles by placing them in `.claude/output-styles/` (without the `clotilde/` subdirectory) and committing them to git.

### `clotilde incognito [name] [options]`

Start a new incognito session that automatically deletes when you exit. If no name is provided, a random name like "happy-fox" or "brave-wolf" will be generated.

```bash
# Random name (e.g., "clever-owl")
clotilde incognito

# Explicit name
clotilde incognito quick-test

# With model and random name
clotilde incognito --model haiku
```

**Options:** Same as `clotilde start` (except `--incognito` is implicit)

### `clotilde resume [name] [options]`

Resume a session by name. Shorthand flags are passed directly to Claude Code for that invocation.

```bash
clotilde resume auth-feature
clotilde resume auth-feature --fast
clotilde resume auth-feature --accept-edits
```

**Options:** `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast` (see [Shorthand Flags](#shorthand-flags))

### `clotilde list`

List all sessions with details (name, model, last used).

### `clotilde inspect <name>`

Show detailed session information including files, settings, context sources, and Claude Code data status.

```bash
clotilde inspect auth-feature
```

### `clotilde fork <parent> [name] [options]`

Fork a session to try different approaches without losing the original.

If no name is provided for incognito forks, a random name will be generated.

```bash
clotilde fork auth-feature auth-experiment

# Create incognito fork with explicit name
clotilde fork auth-feature temp-experiment --incognito

# Create incognito fork with random name (e.g., "clever-owl")
clotilde fork auth-feature --incognito
```

**Options:**
- `--incognito` - Create fork as incognito session (auto-deletes on exit)
- `--accept-edits`, `--yolo`, `--plan`, `--dont-ask`, `--fast` - Shorthand flags (see [Shorthand Flags](#shorthand-flags))

**Note:** You cannot fork FROM incognito sessions, but you can fork TO incognito sessions.

### `clotilde delete <name> [--force]`

Delete a session and all associated Claude Code data (transcripts, agent logs).

**Options:**
- `--force, -f` - Skip confirmation prompt

### `clotilde completion <shell>`

Generate shell completion scripts (bash, zsh, fish, powershell). See `clotilde completion --help` for setup instructions.

## Related Work

Session naming is a wildly requested feature for Claude Code. Multiple open issues track this:

- [#11408](https://github.com/anthropics/claude-code/issues/11408) - Named sessions for easier identification (5+ 👍)
- [#7441](https://github.com/anthropics/claude-code/issues/7441) - `/rename` command to update conversation titles (16+ 👍)
- [#2112](https://github.com/anthropics/claude-code/issues/2112) - Custom session naming with CLI flags
- [#11785](https://github.com/anthropics/claude-code/issues/11785) - Label/tag sessions for context management (High Priority)
- [#11601](https://github.com/anthropics/claude-code/issues/11601), [#11694](https://github.com/anthropics/claude-code/issues/11694) - Closed as duplicates

### Existing solutions

These are the projects I found that are similar to this one:

- [**tweakcc**](https://github.com/Piebald-AI/tweakcc) - Patches Claude Code binaries to add `/title` command, custom system prompts, themes, and more
- [**claude-code-session-name**](https://github.com/richardkmichael/claude-code-session-name) - Python wrapper adding `--session-name` flag, stores names in SQLite

Clotilde is different because:

- Non-invasive (doesn't modify Claude Code)
- Native Go binary (easy distribution, no runtime dependencies)
- Session forking support
- Built-in cleanup (deletes sessions + associated Claude data)

## Development

```bash
# Build
make build

# Run tests
make test

# Run tests with coverage
make coverage

# Run tests in watch mode
make test-watch

# Format code
make fmt

# Lint
make lint

# Install locally to ~/.local/bin
make install
```

**Requirements:**
- Go 1.25+
- Make
- golangci-lint v2.x (for linting - matches CI)

**Setup golangci-lint:**
```bash
# Install golangci-lint v2.x (required for consistent linting with CI)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Verify installation
golangci-lint --version  # Should show v2.x
```

**Note:** The `make lint` command will warn you if your local golangci-lint version doesn't match CI (v2.x).

## License

MIT
