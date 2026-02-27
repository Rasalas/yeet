# 001: Subcommand vs. Commit Message Disambiguation

**Status:** Accepted
**Date:** 2025-02-27

## Context

`yeet` accepts free-text commit messages as positional args (`yeet fix the bug`). It also has subcommands (`config`, `auth`, `log`, `prompt`). This creates ambiguity: does `yeet config` open the config TUI or commit with message "config"?

## Decision

Three-layer approach:

1. **Single word matches subcommand** → runs the subcommand. `yeet config` opens the TUI.
2. **Subcommand + extra args** → treated as commit message. `yeet config the app` commits with message "config the app". Each subcommand checks for unexpected args and delegates to `RunAsCommit()`.
3. **`-m` flag as escape hatch** → `yeet -m "config"` commits with message "config".

## Alternatives Considered

- **Requiring quotes for messages**: Shell strips quotes before the binary sees them, so `yeet "config"` and `yeet config` are identical to Go. Not viable.
- **Word count heuristic**: Single word = subcommand, multiple words = message. Breaks `yeet auth set anthropic` which is a valid subcommand chain.
- **`--` separator**: `yeet -- config` works with cobra but is unintuitive for most users.

## Consequences

- Commit messages that are a single word matching a subcommand name require `-m`. This affects exactly: `config`, `auth`, `log`, `prompt`. All are terrible commit messages anyway.
- Multi-word messages starting with a subcommand name work naturally.
