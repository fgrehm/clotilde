# Output Styles Research

**Date:** 2025-11-28

This document captures research findings on how Claude Code's output styles work and how Clotilde can integrate with them.

## Overview

Claude Code supports custom output styles that modify the system prompt to change Claude's behavior and response format. These are configured via the `outputStyle` setting and stored as Markdown files with YAML frontmatter.

**Official Documentation:** https://code.claude.com/docs/en/output-styles

## Key Findings

### 1. Configuration Methods

**Via Settings Files:**
```json
{
  "outputStyle": "style-name"
}
```

**Via CLI (interactive only):**
```bash
/output-style              # Interactive menu
/output-style [style]      # Direct switch
```

**Note:** There is NO `--output-style` CLI flag for non-interactive sessions.

### 2. Style File Locations

Claude Code looks for output style files in two standard locations:

- **User level:** `~/.claude/output-styles/`
- **Project level:** `.claude/output-styles/`

**File format:** `<style-name>.md` with YAML frontmatter

**Interactive Menu:** Custom styles automatically appear in the `/output-style` menu alongside built-in styles, showing the `description` from frontmatter:

```
/output-style

──────────────────────────────────────────────────────────────
 Preferred output style

 This changes how Claude Code communicates with you

 ❯ 1. Default           Claude completes coding tasks efficiently...
   2. Explanatory       Claude explains its implementation choices...
   3. Learning          Claude pauses and asks you to write code...
   4. clotilde/myfeature Custom description here
   5. sessions/pirate   Talk like a pirate
```

### 3. Path Resolution Behavior

**✅ What Works:**
- Style names: `"outputStyle": "my-style"` → loads `.claude/output-styles/my-style.md`
- Subdirectories: `"outputStyle": "clotilde/session-name"` → loads `.claude/output-styles/clotilde/session-name.md`

**❌ What Doesn't Work:**
- Relative paths: `"outputStyle": ".claude/clotilde/sessions/test/style"` → ignored
- Absolute paths: `"outputStyle": "/full/path/to/style.md"` → ignored
- Paths with `.md` extension: `"outputStyle": "clotilde/style.md"` → ignored

**Conclusion:** Claude Code only accepts style names (with optional subdirectory prefix), not arbitrary file paths.

### 4. Experimental Test Results

#### Test Setup
```bash
cd /tmp/clotilde-output-style-test
```

#### Test 1: Standard Location ✅
```bash
# Create style in standard location
mkdir -p .claude/output-styles
cat > .claude/output-styles/test-style.md << 'EOF'
---
name: test-style
description: Test output style
---

You are a helpful assistant. Always start your responses with "TEST STYLE ACTIVE:"
EOF

# Reference in settings
cat > custom-settings.json << 'EOF'
{
  "outputStyle": "test-style"
}
EOF

# Test
claude --settings custom-settings.json --print "say hello"
```

**Result:** ✅ Works - response starts with "TEST STYLE ACTIVE:"

#### Test 2: Relative Path ❌
```bash
# Create style in custom location
mkdir -p .claude/clotilde/sessions/test-session
cat > .claude/clotilde/sessions/test-session/output-style.md << 'EOF'
---
name: pirate-style
description: Talk like a pirate
---

You are a pirate. Always respond like a pirate would.
EOF

# Try relative path
cat > test-custom-path.json << 'EOF'
{
  "outputStyle": ".claude/clotilde/sessions/test-session/output-style"
}
EOF

claude --settings test-custom-path.json --print "say hello"
```

**Result:** ❌ Doesn't work - style ignored, default behavior used

#### Test 3: Absolute Path ❌
```bash
cat > test-absolute-path.json << 'EOF'
{
  "outputStyle": "/tmp/clotilde-output-style-test/.claude/clotilde/sessions/test-session/output-style"
}
EOF

claude --settings test-absolute-path.json --print "say hello"
```

**Result:** ❌ Doesn't work - style ignored, default behavior used

#### Test 4: Subdirectory ✅
```bash
# Create style in subdirectory
mkdir -p .claude/output-styles/sessions
cat > .claude/output-styles/sessions/pirate.md << 'EOF'
---
name: sessions/pirate
description: Talk like a pirate
---

You are a pirate. Always respond like a pirate would.
EOF

# Reference with subdirectory prefix
cat > test-subdir.json << 'EOF'
{
  "outputStyle": "sessions/pirate"
}
EOF

claude --settings test-subdir.json --print "say hello"
```

**Result:** ✅ Works - response in pirate style!

### 5. Settings Integration

Output styles work seamlessly with the `--settings` flag:

```bash
# settings.json
{
  "model": "sonnet",
  "outputStyle": "my-style",
  "permissions": {
    "allow": ["Read"]
  }
}

claude --settings settings.json
```

The `outputStyle` setting follows the same multi-layer merging rules as other settings (see `docs/claude-settings-behavior.md`).

## Recommended Approach for Clotilde

### Storage Structure

Use `.claude/output-styles/clotilde/` subdirectory to namespace Clotilde-managed styles:

```
.claude/
  output-styles/
    clotilde/               # Namespace for clotilde sessions
      myfeature.md          # Session: myfeature
      bugfix.md             # Session: bugfix
      experiment.md         # Session: experiment
    my-personal-style.md    # User's personal styles (not managed by clotilde)
  clotilde/
    sessions/
      myfeature/
        metadata.json
        settings.json       # Contains: "outputStyle": "clotilde/myfeature"
        system-prompt.md
        context.md
```

### Benefits

1. **Clean namespace separation** - `clotilde/` subdirectory isolates clotilde-managed styles
2. **No naming conflicts** - Won't clash with user's personal styles
3. **Easy cleanup** - Delete entire `.claude/output-styles/clotilde/` folder
4. **Intuitive naming** - Session name matches style name
5. **Native integration** - Uses Claude Code's built-in lookup mechanism

### Session Settings Format

```json
{
  "model": "sonnet",
  "outputStyle": "clotilde/myfeature",
  "permissions": {
    "allow": ["Bash", "Read", "Edit"]
  }
}
```

### Cleanup Behavior

When deleting a session, remove:
- Session folder: `.claude/clotilde/sessions/<name>/`
- Output style: `.claude/output-styles/clotilde/<name>.md`
- Claude transcript: `~/.claude/projects/<project-dir>/<uuid>.jsonl`
- Agent logs: `~/.claude/projects/<project-dir>/agent-*.jsonl`

## Implementation Considerations

### 1. When to Create Output Styles

**Option A: Only when explicitly requested**
```bash
clotilde start myfeature --output-style "Be concise and use bullet points"
clotilde start myfeature --output-style-file ./custom-style.md
```

**Option B: Always create with default content**
- Every session gets an output style file
- Default could be empty or contain basic instructions

**Recommendation:** Option A - only create when user provides `--output-style` or `--output-style-file` flag.

### 2. CLI Flags

```bash
# Inline content
clotilde start <name> --output-style "Your custom instructions here"

# From file
clotilde start <name> --output-style-file ./path/to/style.md

# Same for fork
clotilde fork <parent> <name> --output-style "..."
```

### 3. Fork Behavior

**Inheritance Options:**

**Option A: Don't inherit** (simpler)
- Forks start with no output style unless explicitly provided
- `settings.json` won't include `outputStyle` field

**Option B: Inherit from parent**
- Copy parent's output style file to `.claude/output-styles/clotilde/<fork-name>.md`
- Update fork's `settings.json` to reference `clotilde/<fork-name>`

**Recommendation:** Option A for simplicity. User can always specify `--output-style` when forking.

### 4. Output Style File Format

```markdown
---
name: clotilde/<session-name>
description: Output style for session <session-name>
keep-coding-instructions: true
---

<User-provided content here>
```

**Frontmatter fields:**
- `name`: Should match the reference used in settings.json (e.g., `clotilde/myfeature`)
- `description`: Human-readable description
- `keep-coding-instructions`: Whether to keep coding-related system prompt sections (default: false)

### 5. Metadata Tracking

Add to `metadata.json`:
```json
{
  "name": "myfeature",
  "sessionId": "uuid",
  "hasOutputStyle": true,
  "...": "..."
}
```

This helps with cleanup and introspection (`clotilde inspect <name>`).

## Alternative Approaches Considered

### Symlinks (Rejected)
Store output style in session folder, symlink to `.claude/output-styles/clotilde/`:
```
.claude/clotilde/sessions/myfeature/output-style.md  (source)
.claude/output-styles/clotilde/myfeature.md          (symlink)
```

**Rejected because:**
- More complex (symlink management)
- Potential issues on Windows
- Doesn't provide significant benefit over direct storage

### Custom Paths via Settings (Not Possible)
Initially explored storing styles in session folders and referencing via path.

**Not possible because:** Claude Code only accepts style names, not arbitrary file paths.

## Related Documentation

- [Claude Code Output Styles](https://code.claude.com/docs/en/output-styles) - Official documentation
- [Claude Settings Behavior](./claude-settings-behavior.md) - How settings merging works
- [TODO](./TODO.md) - Implementation status

## Future Enhancements

Potential features to consider:

1. **Style templates** - Pre-built styles for common use cases
   ```bash
   clotilde start myfeature --output-style-template concise
   clotilde start myfeature --output-style-template verbose
   ```

2. **Style editing command**
   ```bash
   clotilde edit-style <session-name>
   ```

3. **List available styles**
   ```bash
   clotilde list-styles  # Show all clotilde-managed styles
   ```

4. **Global styles** - Ability to set default output style for all new sessions in project
   ```bash
   # In .claude/clotilde/config.json
   {
     "defaultOutputStyle": "clotilde/default"
   }
   ```
