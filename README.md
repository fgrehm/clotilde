# Clotilde

Named sessions, profiles, and context management for Claude Code.

> **Documentation**: For the latest stable release, see [Clotilde on the `stable` branch](https://github.com/fgrehm/clotilde/tree/stable). This `main` branch contains development changes and may include features not yet released.

## About the Name

A traditional Brazilian name - sometimes considered old-fashioned or humorous - which adds a light, unpretentious personality to the tool. Pronounced more or less like **KLOH-teel-dee** (Portuguese-ish).

## Why?

Claude Code is great for single conversations, but as you start juggling multiple parallel sessions in the same project, things get painful:

- **Session naming is unreliable**: Claude Code has `/rename`, but names don't persist across resumes, can't be used with `--resume <name>`, and get overwritten by auto-generated slugs. There's no way to name a session at startup either.
- **The resume picker doesn't scale**: Fine for 2-3 sessions, but with more it shows unhelpful text like "This session is being continued from a previous conversation that ran out of context..." Which one was your auth work again?
- **No per-session configuration**: Want to use Haiku for a quick question and Opus for deep feature work? You have to change settings globally or pass flags every time.
- **No persistent context injection**: There's no built-in way to feed the same background context (ticket info, project conventions, task goals) into every session automatically.

## What Clotilde does

Clotilde is a thin wrapper around Claude Code that adds three things: named sessions, profiles, and context management.

```bash
# Named sessions you can instantly switch between
clotilde start auth-feature
clotilde start bugfix-db-timeout
clotilde resume auth-feature

# Fork a session to explore a tangent
clotilde fork auth-feature auth-alternative-approach

# Incognito sessions that auto-delete on exit
clotilde incognito quick-question

# Profiles for different workflows
clotilde start review --profile strict     # deny Bash/Write permissions
clotilde start spike --profile quick       # use Haiku, bypass permissions
```

Session names are stored as external name-to-UUID mappings, so they work reliably regardless of Claude Code's internal naming behavior. Profiles let you define reusable presets (model, permissions, output style) in a single config file. Context files are automatically injected at session start via hooks.

## How It Works

Clotilde never patches or modifies Claude Code.

- Each session is a folder in `.claude/clotilde/sessions/<name>/` containing metadata, optional settings, and system prompt files
- SessionStart hooks handle fork registration, `/clear` UUID tracking, and context injection
- Claude Code is invoked with `--session-id`, `--resume`, `--settings`, and `--append-system-prompt-file` flags

**Note on worktrees:** Since `.claude/clotilde/` lives in each worktree's `.claude/` directory, each worktree gets its own independent sessions and context. Use worktrees for major branches/features, use Clotilde for managing multiple conversations within each worktree.

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

Incognito sessions automatically delete themselves when you exit. See [`clotilde incognito`](#clotilde-incognito-name-options) for usage.

**Note:** Cleanup runs on normal exit (Ctrl+D, `/exit`). If the process crashes or is killed (SIGKILL), the session may persist. Use `clotilde delete <name>` to clean up manually.

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

## Session Profiles

Define named presets in `.claude/clotilde/config.json` for common session configurations. Use `--profile <name>` when creating sessions to apply a profile's settings.

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

**Profile fields:**
- `model` - Claude model (haiku, sonnet, opus)
- `permissionMode` - Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)
- `permissions` - Granular permissions:
  - `allow` - Tools Claude can always use
  - `deny` - Tools Claude cannot use
  - `ask` - Tools that require approval
  - `additionalDirectories` - Extra directories Claude can access
  - `defaultMode` - Default action for tools not in allow/deny/ask (ask, allow, or deny)
  - `disableBypassPermissionsMode` - Disable permission bypass (boolean)
- `outputStyle` - Output style (built-in or custom name)

**Usage examples:**

```bash
# Apply "quick" profile (haiku + bypass permissions)
clotilde start spike --profile quick

# Apply "strict" profile (deny Bash/Write, ask mode)
clotilde start sandboxed --profile strict

# Combine profiles with CLI flags (CLI flags win)
clotilde start research --profile quick --model sonnet

# Use profile on incognito sessions
clotilde incognito --profile quick
```

**Precedence:** Profile values are applied first, then CLI flags override individual fields. For example, `--profile quick --model opus` uses the quick profile's settings but replaces its model with opus.

## Output Styles

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

# With profile
clotilde start spike --profile quick
clotilde start sandboxed --profile strict

# With custom model
clotilde start bugfix --model haiku

# Profile + CLI flag override
clotilde start research --profile quick --model sonnet

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
- `--profile <name>` - Named profile from config (applies model, permissions, and output style as baseline)
- `--model <model>` - Model to use (haiku, sonnet, opus). CLI flag overrides profile.
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
- `--permission-mode <mode>` - Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan). CLI flag overrides profile.
- `--allowed-tools <tools>` - Comma-separated list of allowed tools (e.g. `Bash(npm:*),Read`). CLI flag overrides profile.
- `--disallowed-tools <tools>` - Comma-separated list of disallowed tools (e.g. `Write,Bash(git:*)`). CLI flag overrides profile.
- `--add-dir <directories>` - Additional directories to allow tool access to. CLI flag overrides profile.
- `--output-style <style>` - Output style: `default`, `Explanatory`, `Learning`, existing style name, or custom content. CLI flag overrides profile.
- `--output-style-file <path>` - Path to custom output style file. CLI flag overrides profile.

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

Claude Code has a `/rename` command, but session names don't persist reliably across resumes, can't be used with `--resume <name>`, and there's no `--session-name` flag for naming at startup. Several open issues track these gaps:

- [#25870](https://github.com/anthropics/claude-code/issues/25870) - `--session-name` flag for naming sessions at startup
- [#26249](https://github.com/anthropics/claude-code/issues/26249) - `/rename` names not indexed, can't resume by name
- [#26240](https://github.com/anthropics/claude-code/issues/26240) - Session names lost after resuming and continuing work
- [#11408](https://github.com/anthropics/claude-code/issues/11408) - Named sessions for easier identification

### Existing solutions

- [**tweakcc**](https://github.com/Piebald-AI/tweakcc) - Patches Claude Code to add custom system prompts, toolsets, themes, and more
- [**claude-code-session-name**](https://github.com/richardkmichael/claude-code-session-name) - Python wrapper adding `--session-name` flag, stores names in SQLite

Clotilde is different because:

- Non-invasive (doesn't patch or modify Claude Code)
- Native Go binary (no runtime dependencies)
- Goes beyond naming: profiles, context injection, incognito sessions, forking
- Built-in cleanup (deletes sessions + associated Claude Code data)

## Development

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

**Configure git hooks** (recommended):
```bash
make setup-hooks
```

This enables pre-commit checks that automatically format and lint your code before committing, preventing CI failures.

**Build & Test:**

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

**Note:** The `make lint` command will warn you if your local golangci-lint version doesn't match CI (v2.x).

## License

MIT
