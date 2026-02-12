# Quality-of-Life Shorthand Flags

## Problem

Common Claude Code flags like `--permission-mode acceptEdits` are verbose to type. Clotilde can provide shorter, more memorable aliases that translate to their underlying Claude Code equivalents, making frequent workflows faster.

## Design Principle

These are **syntactic sugar only**. Each shorthand flag maps directly to an existing Claude Code flag or Clotilde `--permission-mode` value. No new behavior is introduced. Shorthands and their long-form equivalents are mutually exclusive (error if both provided).

All shorthand flags apply to `start`, `incognito`, `fork`, and `resume` commands unless noted otherwise.

## Shorthand Flags

### Permission Mode Shortcuts

These translate to `--permission-mode <value>` and conflict with each other and with `--permission-mode`.

| Shorthand | Translates To | Description |
|-----------|--------------|-------------|
| `--accept-edits` | `--permission-mode acceptEdits` | Auto-approve file edits, ask for everything else |
| `--yolo` | `--permission-mode bypassPermissions` | Skip all permission checks |
| `--plan` | `--permission-mode plan` | Plan mode (Claude proposes, you approve) |
| `--dont-ask` | `--permission-mode dontAsk` | Approve everything without asking |

**Examples:**
```bash
clotilde start refactor --accept-edits
clotilde incognito --yolo
clotilde start spike --plan --model opus
clotilde resume my-session --dont-ask
```

**Conflict rules:**
- Using more than one permission shorthand in the same command is an error
- Using a shorthand together with `--permission-mode` is an error

### Effort Level Shortcut

Claude Code supports `--effort <level>` (low, medium, high) but it's a pass-through flag today (requires `--`). Promoting it to a first-class Clotilde flag makes it discoverable and easier to use.

| Shorthand | Translates To | Description |
|-----------|--------------|-------------|
| `--effort <level>` | `--effort <level>` (pass-through) | Set reasoning effort (low, medium, high) |

Unlike permission mode shortcuts, this isn't stored in settings. It's passed directly to the `claude` CLI invocation as an additional arg.

**Examples:**
```bash
clotilde start quick-check --effort low --model haiku
clotilde resume deep-dive --effort high
```

### MCP Config Shortcut

Loading MCP server configs currently requires `-- --mcp-config path/to/config.json`. Promoting to first-class flag.

| Shorthand | Translates To | Description |
|-----------|--------------|-------------|
| `--mcp-config <path>` | `--mcp-config <path>` (pass-through) | Load MCP servers from a JSON file |

Pass-through to `claude` CLI, not stored in settings.

**Examples:**
```bash
clotilde start with-tools --mcp-config ./my-servers.json
clotilde resume my-session --mcp-config ~/.config/mcp/default.json
```

### Debug Shortcut

Currently requires `-- --debug [filter]`. Promoting to first-class flag.

| Shorthand | Translates To | Description |
|-----------|--------------|-------------|
| `--debug [filter]` | `--debug [filter]` (pass-through) | Enable debug mode with optional category filter |

Pass-through to `claude` CLI, not stored in settings.

**Examples:**
```bash
clotilde start my-session --debug
clotilde start my-session --debug api,hooks
clotilde resume my-session --debug mcp
```

## Implementation

### Where changes happen

1. **Flag registration** (`cmd/start.go`, `cmd/incognito.go`, `cmd/fork.go`, `cmd/resume.go`): Add new flags to each command.

2. **Flag resolution** (`cmd/session_create.go`): For permission shortcuts, resolve them into the `PermissionMode` field with conflict detection.

3. **Pass-through handling** (`cmd/start.go`, `cmd/resume.go`, `cmd/fork.go`, `cmd/incognito.go`): For `--effort`, `--mcp-config`, and `--debug`, append to `additionalArgs` before invoking claude.

### Permission shorthand resolution

```go
func resolvePermissionMode(cmd *cobra.Command) (string, error) {
    explicit, _ := cmd.Flags().GetString("permission-mode")
    acceptEdits, _ := cmd.Flags().GetBool("accept-edits")
    yolo, _ := cmd.Flags().GetBool("yolo")
    plan, _ := cmd.Flags().GetBool("plan")
    dontAsk, _ := cmd.Flags().GetBool("dont-ask")

    count := 0
    mode := explicit
    if acceptEdits { count++; mode = "acceptEdits" }
    if yolo { count++; mode = "bypassPermissions" }
    if plan { count++; mode = "plan" }
    if dontAsk { count++; mode = "dontAsk" }

    if count > 1 || (count == 1 && explicit != "") {
        return "", fmt.Errorf("cannot combine multiple permission mode flags")
    }
    return mode, nil
}
```

### Pass-through flag collection

```go
func collectPassthroughFlags(cmd *cobra.Command, additionalArgs []string) []string {
    if effort, _ := cmd.Flags().GetString("effort"); effort != "" {
        additionalArgs = append(additionalArgs, "--effort", effort)
    }
    if mcpConfig, _ := cmd.Flags().GetString("mcp-config"); mcpConfig != "" {
        additionalArgs = append(additionalArgs, "--mcp-config", mcpConfig)
    }
    if cmd.Flags().Changed("debug") {
        filter, _ := cmd.Flags().GetString("debug")
        if filter != "" {
            additionalArgs = append(additionalArgs, "--debug", filter)
        } else {
            additionalArgs = append(additionalArgs, "--debug")
        }
    }
    return additionalArgs
}
```

### Scope per command

| Flag | `start` | `incognito` | `fork` | `resume` |
|------|---------|-------------|--------|----------|
| `--accept-edits` | yes | yes | yes | yes |
| `--yolo` | yes | yes | yes | yes |
| `--plan` | yes | yes | yes | yes |
| `--dont-ask` | yes | yes | yes | yes |
| `--effort` | yes | yes | yes | yes |
| `--mcp-config` | yes | yes | yes | yes |
| `--debug` | yes | yes | yes | yes |

## What This Does NOT Include

- **`--print` / `-p`**: Scripting/pipe mode. Different enough usage pattern that pass-through via `--` is fine.
- **`--continue` / `-c`**: Clotilde manages sessions by name, so "continue most recent" doesn't map cleanly. Could be a separate command later.
- **`--verbose`**: Already a Clotilde global flag.
- **`--model`**: Already a first-class Clotilde flag.
- **`--allowed-tools` / `--disallowed-tools` / `--add-dir`**: Already first-class Clotilde flags.

## Testing

- Unit tests for `resolvePermissionMode`: each shorthand alone, conflicts between shorthands, conflicts with `--permission-mode`
- Unit tests for `collectPassthroughFlags`: each flag, combinations, empty values
- Integration tests: verify the assembled `claude` CLI args include the correct flags
