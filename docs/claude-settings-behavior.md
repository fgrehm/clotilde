# Claude Code Settings Behavior Reference

**Last Updated:** 2025-11-28

This document details how Claude Code's `--settings` flag and multi-layer settings system works, based on empirical testing. Understanding this is critical for Clotilde's session management design.

## Table of Contents

- [Quick Summary](#quick-summary)
- [Settings Layers](#settings-layers)
- [Model Selection Behavior](#model-selection-behavior)
- [Permission Behavior](#permission-behavior)
- [Permission Resolution Algorithm](#permission-resolution-algorithm)
- [Implications for Clotilde](#implications-for-clotilde)
- [Detailed Test Results](#detailed-test-results)

---

## Quick Summary

### Critical Findings

1. **Model selection uses precedence:** CLI > Local > Project > Global (highest wins)
2. **Output style uses precedence:** CLI > Local > Project > Global (same as model)
3. **Permissions merge across layers:** Allow rules combine (union)
4. **Deny is absolute:** Any `deny` from ANY layer blocks the operation
5. **Interactive changes to `.claude/settings.local.json`:** Permission approvals and `/output-style` changes save here
6. **Shared state:** All sessions in same directory share `.claude/settings.local.json`

### Settings Layers (in priority order)

1. **Global:** `~/.claude/settings.json`
2. **Project:** `.claude/settings.json`
3. **Local:** `.claude/settings.local.json` (auto-created by approvals)
4. **CLI:** `--settings <file>` flag (highest priority)

---

## Model Selection Behavior

**Strategy:** Precedence (highest layer wins)

| Layer | Priority | Example |
|-------|----------|---------|
| CLI `--settings` | 1 (highest) | `{"model": "opus"}` |
| Local `.claude/settings.local.json` | 2 | `{"model": "sonnet"}` |
| Project `.claude/settings.json` | 3 | `{"model": "haiku"}` |
| Global `~/.claude/settings.json` | 4 (lowest) | `{"model": "sonnet"}` |

**Result:** If all layers specify a model, CLI wins. If CLI doesn't specify, Local wins, etc.

**Tested scenarios:**
- ✅ Global only → Uses global
- ✅ Global + Project → Uses project
- ✅ Global + CLI → Uses CLI
- ✅ All layers → Uses CLI

---

## Output Style Behavior

**Strategy:** Precedence (same as model)

| Layer | Priority | Example |
|-------|----------|---------|
| CLI `--settings` | 1 (highest) | `{"outputStyle": "Explanatory"}` |
| Local `.claude/settings.local.json` | 2 | `{"outputStyle": "Learning"}` |
| Project `.claude/settings.json` | 3 | `{"outputStyle": "default"}` |
| Global `~/.claude/settings.json` | 4 (lowest) | `{"outputStyle": "default"}` |

**Result:** Highest layer wins (same precedence as model).

### Built-in Output Styles

Claude Code has 3 built-in output styles:

| Style | Value | Description |
|-------|-------|-------------|
| Default | `"default"` | Claude completes coding tasks efficiently and provides concise responses |
| Explanatory | `"Explanatory"` | Claude explains its implementation choices and codebase patterns |
| Learning | `"Learning"` | Claude pauses and asks you to write small pieces of code for hands-on practice |

**Note:** Case matters! `"default"` is lowercase, `"Explanatory"` and `"Learning"` are capitalized.

### Custom Output Styles

Users can create custom styles in:
- User level: `~/.claude/output-styles/<name>.md`
- Project level: `.claude/output-styles/<name>.md`

Subdirectories work: `.claude/output-styles/clotilde/session-name.md` → `"outputStyle": "clotilde/session-name"`

Custom styles automatically appear in the interactive `/output-style` menu.

### Interactive Changes

When a user runs `/output-style` during a session to change the output style:
- The change saves to `.claude/settings.local.json`
- This affects ALL future sessions in the same directory (shared state)
- **However:** CLI `--settings` still takes precedence, so session-specific styles are preserved

### Tested Scenarios

**Test 1: CLI vs Project**
```json
// .claude/settings.json
{"outputStyle": "test-style"}

// cli-settings.json
{"outputStyle": "Explanatory"}
```
```bash
claude --settings cli-settings.json --print "say hello"
# Result: ✅ Uses Explanatory (CLI wins)
```

**Test 2: CLI vs Local**
```json
// .claude/settings.local.json
{"outputStyle": "Learning"}

// cli-settings.json
{"outputStyle": "Explanatory"}
```
```bash
claude --settings cli-settings.json --print "say hello"
# Result: ✅ Uses Explanatory (CLI wins)
```

**Test 3: Interactive Change Storage**
```bash
claude --settings cli-settings.json  # Has "Explanatory"
> /output-style
  → Set to Learning

cat .claude/settings.local.json
# Result: {"outputStyle": "Learning"}
```

### Implications for Clotilde

✅ **Session-specific output styles work perfectly** via CLI `--settings` precedence
✅ **User can override with `/output-style`** and it saves to `.claude/settings.local.json`
✅ **CLI settings still win** on next resume, preserving session-specific style
✅ **Sessions without explicit outputStyle** inherit from `.claude/settings.local.json`

**Example workflow:**
```bash
# Session with custom style
clotilde start session-a --output-style "Be concise"
# → sessions/session-a/settings.json: {"outputStyle": "clotilde/session-a"}
# → .claude/output-styles/clotilde/session-a.md created

# User changes style during session
/output-style → Learning
# → .claude/settings.local.json: {"outputStyle": "Learning"}

# Resume session-a
clotilde resume session-a
# → Still uses "clotilde/session-a" (CLI --settings wins) ✅

# Start session without explicit style
clotilde start session-b
# → Uses "Learning" from .claude/settings.local.json (fallback) ✅
```

---

## Permission Behavior

**Strategy:** Union with veto

### Allow Rules (Additive)

Permissions from all layers **merge** together:

```
CLI:     allow: ["Bash(cat:*)"]
Local:   allow: ["Bash(rm:*)"]
Result:  Both cat AND rm are allowed ✅
```

### Deny Rules (Absolute Veto)

A `deny` from **ANY layer blocks** the operation, regardless of `allow` in other layers:

```
CLI:     allow: ["Bash(rm:*)"]
Local:   deny: ["Bash(rm:*)"]
Result:  rm is BLOCKED 🚫
```

```
CLI:     deny: ["Bash(rm:*)"]
Local:   allow: ["Bash(rm:*)"]
Result:  rm is BLOCKED 🚫
```

**Key insight:** Layer precedence (CLI > Project > Local) does NOT apply to `deny`. Any deny anywhere blocks.

### Approval Storage (Critical!)

**Where permanent approvals are saved:**

| When using | Approvals saved to |
|------------|-------------------|
| `--settings custom.json` | `.claude/settings.local.json` ❌ NOT custom.json |
| No `--settings` flag | `.claude/settings.local.json` |

**Consequences:**
- CLI-provided settings are **read-only** for permissions
- All sessions in same directory **share** `.claude/settings.local.json`
- Cannot isolate per-session approvals using `--settings`

---

## Permission Resolution Algorithm

Based on testing, Claude Code uses this logic:

```python
For each tool request:
  1. Collect ALL deny rules from all layers (Global ∪ Project ∪ Local ∪ CLI)
  2. Collect ALL allow rules from all layers (Global ∪ Project ∪ Local ∪ CLI)
  3. Collect ALL ask rules from all layers (Global ∪ Project ∪ Local ∪ CLI)

  4. If tool matches ANY deny rule → BLOCK (no prompt)
  5. Else if tool matches ANY allow rule → ALLOW
  6. Else if tool matches ANY ask rule → PROMPT
  7. Else → Use default permission mode
```

**Key takeaway:** `deny` acts as a blacklist that cannot be overridden.

---

## Implications for Clotilde

### What Works Perfectly

✅ **Model per session** - Can set different models via `--settings` per session
✅ **Initial permissions** - Can pre-populate permissions in session settings.json

### What's Shared

⚠️ **Ongoing approvals** - All sessions share `.claude/settings.local.json`

### Recommended Design

Based on findings:

1. **Keep session-specific `settings.json`** for model + initial permissions ✅
2. **Accept shared approvals** as a feature (permissions are per-project, not per-conversation) ✅
3. **Add `.claude/settings.local.json` to `.gitignore`** (personal preferences) ✅
4. **Document shared approval behavior** clearly ✅

### Alternative Approaches Considered

**Option 1: Accept shared permissions** (RECOMMENDED)
- Simplest approach
- Actually makes sense: permissions are per-project, not per-conversation
- Document this behavior clearly

**Option 2: Don't provide `--settings` for permissions**
- Remove permissions from session `settings.json`
- Only use `--settings` for model selection
- Let project-wide `.claude/settings.json` handle permissions

**Option 3: Use different working directories** (COMPLEX, NOT RECOMMENDED)
- Set each session's `additionalDirectories` to unique folder
- Permissions isolated per session folder
- Significant UX impact - files wouldn't be in actual project directory

---

## Detailed Test Results

_This section contains the raw experimental data and test procedures used to derive the findings above._

### Test 1: Basic Settings Precedence

**Current Global Settings:**
```json
{
  "permissions": {
    "allow": [],
    "deny": [],
    "ask": ["Bash(git add -A:*)"]
  },
  "alwaysThinkingEnabled": true,
  "model": "sonnet"
}
```

**Test Scenarios:**

#### 1a. Global only (baseline)
- Global: model = "sonnet"
- Project: none
- CLI: none
- **Expected:** sonnet

#### 1b. Global + Project
- Global: model = "sonnet"
- Project: model = "haiku"
- CLI: none
- **Expected:** haiku (project overrides global)

#### 1c. Global + CLI
- Global: model = "sonnet"
- Project: none
- CLI: model = "opus"
- **Expected:** opus (CLI overrides global)

#### 1d. All three layers
- Global: model = "sonnet"
- Project: model = "haiku"
- CLI: model = "opus"
- **Expected:** opus (CLI overrides all)

**Actual Results:**

| Test | Global | Project | CLI | Result |
|------|--------|---------|-----|--------|
| 1a   | sonnet | -       | -   | ✅ **Sonnet 4.5** |
| 1b   | sonnet | haiku   | -   | ✅ **Haiku 4.5** |
| 1c   | sonnet | -       | opus | ✅ **Opus 4.5** |
| 1d   | sonnet | haiku   | opus | ✅ **Opus 4.5** |

**Confirmed Precedence:** CLI `--settings` > Project `.claude/settings.json` > Global `~/.claude/settings.json`

---

### Test 2: Permission Approval Storage (CLI settings provided)

**Setup:**
- Custom settings file: `custom-settings.json` (initially: `{"model": "opus"}`)
- Project settings: `.claude/settings.json` (initially: `{"model": "haiku"}`)
- No `.claude/settings.local.json` initially
- Start interactive session with `--settings custom-settings.json`
- Request a Write operation
- Approve with "don't ask again"

**Question:** Where does the approval get saved?
- Option A: Into `custom-settings.json` (CLI-provided)
- Option B: Into `.claude/settings.json` (project)
- Option C: Into `.claude/settings.local.json` (new file)
- Option D: Global `~/.claude/settings.json`

**Test Steps:**
1. Clean slate: `rm -f .claude/settings.local.json`
2. Backup files: `cp custom-settings.json custom-settings.json.orig && cp .claude/settings.json .claude/settings.json.orig`
3. Start session: `claude --settings custom-settings.json --session-id "$(uuidgen)"`
4. Prompt: "Create a file called test.txt with the word 'hello'"
5. When permission prompt appears, choose "Don't ask about this permission again"
6. Exit and compare files to originals

**Actual:**

❌ **TEST INVALID** - Permission prompt was "allow all edits during this session" (temporary), not "don't ask again" (permanent).

**Lesson:** Write/Edit prompts only offer session-level approval, not permanent settings changes.

**Need to retest with Bash command to get permanent approval option.**

---

### Test 2b: Permission Approval with Bash (CLI settings provided)

**Setup:**
- Clean state (delete test.txt if exists)
- Fresh session with `--settings custom-settings.json`
- Request a Bash command that will trigger permission prompt with "don't ask again" option
- Verify where the approval gets saved

**Test Steps:**
1. Clean up: `rm -f test.txt test2.txt test3.txt`
2. Backup files again: `cp custom-settings.json custom-settings.json.orig && cp .claude/settings.json .claude/settings.json.orig`
3. Check global before: `cp ~/.claude/settings.json ~/settings-global-before.json`
4. Start session: `claude --settings custom-settings.json --session-id "$(uuidgen)"`
5. Prompt: "Run ls -la to show me the current directory contents"
6. When permission prompt appears, choose the permanent "Don't ask about this permission again" option
7. Exit and compare all files

**Actual:**

✅ Permission prompt appeared for `Bash(rm test.txt)`
✅ Selected option: "Yes, and don't ask again for rm commands in /tmp/claude-settings-experiment"

**Files checked after approval:**

| File | Status | Content |
|------|--------|---------|
| `custom-settings.json` (CLI) | ✅ **UNCHANGED** | `{"model": "opus"}` |
| `.claude/settings.json` (project) | ✅ **UNCHANGED** | `{"model": "haiku"}` |
| `.claude/settings.local.json` | 🔥 **CREATED!** | `{"permissions": {"allow": ["Bash(rm:*)"], "deny": [], "ask": []}}` |
| `~/.claude/settings.json` (global) | ✅ **UNCHANGED** | Original permissions intact |

**🎯 KEY FINDING:** When using `--settings custom-settings.json`, permanent approvals are saved to `.claude/settings.local.json`, NOT to the custom settings file provided via CLI!

---

### Test 3: Permission Approval Storage (no CLI settings)

**Setup:**
- Clean `.claude/settings.local.json`
- Start session WITHOUT `--settings` flag
- Approve a tool with "don't ask again"

**Question:** Where does the approval get saved?

**Actual:**

✅ Permission prompt appeared for `Bash(mkdir testdir)`
✅ Selected permanent "don't ask again" option

**Files checked after approval:**

| File | Status | Content |
|------|--------|---------|
| `.claude/settings.json` (project) | ✅ **UNCHANGED** | `{"model": "haiku"}` |
| `.claude/settings.local.json` | 🔥 **CREATED!** | `{"permissions": {"allow": ["Bash(mkdir:*)"], "deny": [], "ask": []}}` |

**🎯 KEY FINDING:** Even without `--settings`, permanent approvals go to `.claude/settings.local.json`!

---

### Test 4: Settings Merging Behavior

**Setup:**
- Test if permissions from different layers merge together or override each other
- Specifically: if CLI `--settings` provides `allow: ["Read"]`, and user approves `Bash(ls:*)` (goes to `.claude/settings.local.json`), are both active?

**Test Steps:**
1. Clean slate: `rm -f .claude/settings.local.json`
2. Create CLI settings with pre-approved permission: `{"permissions": {"allow": ["Read"]}}`
3. Start session with that settings file
4. Trigger a Bash permission prompt, approve permanently
5. Check if BOTH permissions are active (Read from CLI + Bash from local)

**Test Commands:**
```bash
cd /tmp/claude-settings-experiment
rm -f .claude/settings.local.json
echo '{"permissions": {"allow": ["Read"]}}' > merge-test-settings.json
claude --settings merge-test-settings.json --session-id "$(uuidgen)"
```

Prompt: "Run echo 'test' and then read the settings-experiment.md file"

This should:
- Allow `Read` without prompting (from CLI settings)
- Prompt for `Bash(echo:*)`
- After approval, check if both are in effect

**Actual:**

✅ **First session** - `cat` allowed (CLI), `rm` prompted and approved
✅ **Second session** - Both `cat` AND `rm` worked without prompting!

**Files after test:**

| File | Content |
|------|---------|
| `merge-test-settings.json` (CLI) | `{"permissions": {"allow": ["Bash(cat:*)"]}}` |
| `.claude/settings.local.json` | `{"permissions": {"allow": ["Bash(rm:*)"]}}` |

**🎯 KEY FINDING:** Permissions from different layers **MERGE** together!
- CLI settings provided `Bash(cat:*)` ✅
- Local settings had `Bash(rm:*)` ✅
- Both were active simultaneously - no prompts on second run

**Conclusion:** Claude Code merges permissions from all layers (Global → Project → Local → CLI), creating a union of allowed/denied tools.

---

## Findings

### Precedence (Model Selection)

**Confirmed hierarchy:** CLI `--settings` > Project `.claude/settings.json` > Global `~/.claude/settings.json`

- All three layers tested
- Each layer successfully overrides the previous one
- CLI flag has highest priority

### Approval Storage

**Universal rule:** Permanent permission approvals ALWAYS save to `.claude/settings.local.json`

- ✅ With `--settings custom.json` → saves to `.claude/settings.local.json`
- ✅ Without `--settings` → saves to `.claude/settings.local.json`
- ❌ Never saves to CLI-provided settings file
- ❌ Never saves to `.claude/settings.json`
- ❌ Never saves to global `~/.claude/settings.json`

**Implication for Clotilde:** Cannot use `--settings` to isolate session permissions. All sessions in the same directory will share `.claude/settings.local.json` approvals!

### Merging Behavior

**Confirmed:** Permissions from all layers **MERGE**, but with critical rules:

#### Rule 1: `allow` rules merge (union)
**Test scenario:**
- CLI settings: `allow: ["Bash(cat:*)"]`
- Local settings: `allow: ["Bash(rm:*)"]`
- Result: **Both** permissions active simultaneously ✅

#### Rule 2: `deny` is an absolute veto
**Test scenario A (Local denies, CLI allows):**
- CLI settings: `allow: ["Bash(rm:*)"]`
- Local settings: `deny: ["Bash(rm:*)"]`
- Result: **DENIED** 🚫

**Test scenario B (CLI denies, Local allows):**
- CLI settings: `deny: ["Bash(rm:*)"]`
- Local settings: `allow: ["Bash(rm:*)"]`
- Result: **DENIED** 🚫

**Key insight:**
- `allow` rules are additive across layers (union)
- `deny` rules are also collected across layers (union), but they veto any matching `allow`
- Layer precedence (CLI > Project > Local > Global) does NOT apply to `deny`
- A single `deny` from ANY layer blocks the operation, regardless of `allow` in other layers

---

## Conclusions

### Critical Findings

1. **`.claude/settings.local.json` is the approval sink** - All permanent permission approvals go here, regardless of how you start the session

2. **`--settings` is read-only for permissions** - You can provide initial permissions via `--settings`, but new approvals won't save back to that file

3. **Shared permission state** - Multiple sessions in the same directory will all see the same `.claude/settings.local.json` approvals

4. **`allow` permissions merge across layers** - Permissions from all sources combine additively (union)

5. **`deny` is an absolute veto** - A `deny` from ANY layer (Global/Project/Local/CLI) blocks the operation, regardless of `allow` rules in other layers

6. **Model selection uses precedence** - CLI > Project > Global (winner takes all)

### Permission Resolution Algorithm

Based on testing, Claude Code appears to use this logic:

```
For each tool request:
  1. Collect ALL deny rules from all layers (Global ∪ Project ∪ Local ∪ CLI)
  2. Collect ALL allow rules from all layers (Global ∪ Project ∪ Local ∪ CLI)
  3. Collect ALL ask rules from all layers (Global ∪ Project ∪ Local ∪ CLI)
  4. If tool matches ANY deny rule → BLOCK (no prompt)
  5. Else if tool matches ANY allow rule → ALLOW
  6. Else if tool matches ANY ask rule → PROMPT
  7. Else → Use default permission mode
```

**Key takeaway:** `deny` acts as a blacklist that cannot be overridden.

### Implications for Clotilde

**Current design assumption:** Each session has isolated settings in `.claude/clotilde/sessions/<name>/settings.json`

**Reality check:**

✅ **Model setting works as expected** - Can set different models per session via `--settings`, highest priority wins

❌ **Permission isolation is impossible** - All sessions share `.claude/settings.local.json` for permanent approvals

**Consequences:**
1. If user approves `Bash(rm:*)` in session A, it's automatically available in session B (same directory)
2. Cannot have "strict permissions session" and "loose permissions session" in the same project
3. The `settings.json` in each session folder can provide **initial** permissions, but not capture **ongoing** approvals

**Potential solutions:**

**Option 1: Accept shared permissions** (RECOMMENDED)
- Simplest approach
- Actually makes sense: permissions are per-project, not per-conversation
- Document this behavior clearly

**Option 2: Ignore `.claude/settings.local.json`**
- Add to `.gitignore` by default
- Let each developer have their own permission preferences
- Sessions still share within one developer's workspace

**Option 3: Don't provide `--settings` at all**
- Remove `settings.json` from session folders
- Only use `--append-system-prompt-file` for session customization
- Let project-wide `.claude/settings.json` handle all permissions

**Option 4: Use different working directories** (COMPLEX)
- Set each session's `additionalDirectories` to a unique session folder
- Permissions would be isolated per session folder
- Significant UX impact - files wouldn't be in the actual project directory

---

## Summary: Complete Settings Behavior

### Settings Layers (in order)
1. **Global:** `~/.claude/settings.json`
2. **Project:** `.claude/settings.json`
3. **Local:** `.claude/settings.local.json` (auto-created by approvals)
4. **CLI:** `--settings <file>` flag

### Model Selection Behavior
- **Strategy:** Precedence (highest layer wins)
- **Order:** CLI > Project > Local > Global
- **Result:** Single model selected from highest layer that specifies it

### Permission Behavior
- **Strategy:** Union with veto
- **Allow rules:** Merge across all layers (union)
- **Deny rules:** Merge across all layers (union), then veto any matching allow
- **Ask rules:** Merge across all layers (union)
- **Approval storage:** Always goes to `.claude/settings.local.json`

### Key Constraints
1. **Cannot override deny:** Once ANY layer denies a permission, it cannot be allowed by any other layer
2. **Cannot isolate approvals:** All sessions in same directory share `.claude/settings.local.json`
3. **CLI settings are read-only:** Approvals never save back to `--settings` file
4. **Local always writable:** `.claude/settings.local.json` is always created/updated for approvals

### Recommended Approach for Clotilde

Based on these findings:

1. **Keep model setting per session** ✅ (works perfectly via CLI precedence)
2. **Accept shared permissions** ✅ (simplest, actually makes sense)
3. **Pre-populate initial permissions** ✅ (via session settings.json)
4. **Document the shared approval behavior** ✅ (critical for user understanding)
5. **Add `.claude/settings.local.json` to `.gitignore`** ✅ (personal preferences, not committed)

---

## Test 5: Conflicting Permissions (Deny vs Allow)

### Test 5a: Local denies, CLI allows

**Question:** If `.claude/settings.local.json` has `deny: ["Bash(rm:*)"]` and CLI settings have `allow: ["Bash(rm:*)"]`, which wins?

**Setup:**
```bash
cd /tmp/claude-settings-experiment
echo '{"permissions": {"allow": [], "deny": ["Bash(rm:*)"], "ask": []}}' > .claude/settings.local.json
echo '{"permissions": {"allow": ["Bash(rm:*)"]}}' > conflict-test-cli.json
echo "test file" > testfile.txt
```

**Test:** Start session with CLI settings and try to remove file
```bash
claude --settings conflict-test-cli.json --session-id "$(uuidgen)"
```

Prompt: "Remove testfile.txt"

**Expected outcomes:**
- **Deny wins:** Will prompt or refuse
- **Allow wins:** Will execute without prompt
- **Precedence wins:** CLI layer overrides local layer

**Actual:**

🚫 **DENY WINS!** - Command was blocked despite CLI `allow`

**Observations:**
- Claude attempted `Bash(rm testfile.txt)` multiple times
- Each attempt failed with: "Permission to use Bash with command rm testfile.txt has been denied"
- CLI settings had `allow: ["Bash(rm:*)"]`
- Local settings had `deny: ["Bash(rm:*)"]`
- The `deny` from local completely overrode the `allow` from CLI

**Conclusion:** `deny` takes precedence over `allow`, regardless of which settings layer it comes from!

---

### Test 5b: CLI denies, Local allows

**Question:** If CLI settings have `deny: ["Bash(rm:*)"]` and `.claude/settings.local.json` has `allow: ["Bash(rm:*)"]`, which wins?

**Setup:**
```bash
cd /tmp/claude-settings-experiment
echo '{"permissions": {"allow": ["Bash(rm:*)"], "deny": [], "ask": []}}' > .claude/settings.local.json
echo '{"permissions": {"deny": ["Bash(rm:*)"]}}' > conflict-test-cli2.json
echo "test file 2" > testfile2.txt
```

**Test:**
```bash
claude --settings conflict-test-cli2.json --session-id "$(uuidgen)"
```

Prompt: "Remove testfile2.txt"

**Actual:**

🚫 **DENY WINS AGAIN!** - Command was blocked despite Local `allow`

**Observations:**
- Claude attempted `Bash(rm testfile2.txt)` and `Bash(rm -f testfile2.txt)`
- Both failed with: "Permission to use Bash with command rm... has been denied"
- Local settings had `allow: ["Bash(rm:*)"]`
- CLI settings had `deny: ["Bash(rm:*)"]`
- The `deny` from CLI completely overrode the `allow` from local

**🎯 CRITICAL RULE DISCOVERED:**

**`deny` is an absolute veto - it wins over `allow` from ANY layer, regardless of precedence!**

This means:
- Global `deny` + CLI `allow` = DENIED
- Project `deny` + Local `allow` = DENIED
- ANY layer with `deny` blocks the operation
- Layer hierarchy (CLI > Project > Local > Global) does NOT apply to `deny`
- `deny` is processed as a union across all layers, and any match blocks execution
