# Clotilde Documentation

This directory contains in-depth documentation for understanding Clotilde's design and the underlying Claude Code behavior.

## Available Documents

### [Roadmap](ROADMAP.md)

Current status, known limitations, and future ideas.

### [Claude Settings Behavior](claude-settings-behavior.md)

Analysis of how Claude Code's settings system works, including:
- Multi-layer settings resolution (Global → Project → Local → CLI)
- Model selection precedence
- Permission merging rules
- The critical finding that approvals always save to `.claude/settings.local.json`

**Why this matters for Clotilde:** Understanding these behaviors is essential for designing session isolation, permission handling, and the overall session management strategy.

---

## Contributing Documentation

When adding new documentation:
1. Put detailed technical analysis here in `docs/`
2. Reference it from the main [`CLAUDE.md`](../CLAUDE.md) file
3. Keep docs focused on "why" and "how it works" rather than "how to use"
