# 002: Inline Editor + $EDITOR for Message Editing

**Status:** Accepted
**Date:** 2025-02-27

## Context

After the AI generates a commit message (or the user provides one), they need a way to tweak it before committing. The question is how to offer editing.

## Decision

Two options via keypress at the confirm prompt:

- **`e`** — Inline editor. Edits the message directly in the terminal with cursor keys, backspace, Ctrl+A/E/U. Best for small tweaks (fix a word, change scope).
- **`E` (Shift)** — Opens `$VISUAL` / `$EDITOR` / `vi` (fallback chain) with the message in a temp file. For when you want full editor power.

Both return to the confirm prompt after editing. Esc in either cancels the edit and keeps the original message.

## Alternatives Considered

- **Only $EDITOR**: Git-standard approach, but opening vim for a one-line message is overkill. Most edits are "change one word".
- **Only inline**: Insufficient for users who want multi-line messages or are faster in their editor.
- **Configurable (pick one in config)**: Unnecessary complexity. Both keys are always available, user picks in the moment.

## Consequences

- The inline editor handles basic ASCII. No multi-byte Unicode support in raw mode, but sufficient for commit messages.
- `$EDITOR` supports multi-line — the first line of the file becomes the commit message.
