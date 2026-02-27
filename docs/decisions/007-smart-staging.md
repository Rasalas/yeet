# 007: Smart Staging — Respect Existing Staged Changes

**Status:** Accepted
**Date:** 2025-02-27

## Context

The original flow always ran `git add --all` before committing. This ignores any intentional partial staging the user may have done (e.g., `git add src/auth.go` to commit only one file).

## Decision

Check for existing staged changes before auto-staging:

1. **Staged changes exist** → use them as-is, do not run `git add --all`
2. **Nothing staged** → run `git add --all` (original behavior)

On cancel (Escape), only unstage (`git reset`) if yeet auto-staged. If the user staged manually, their staging is preserved on cancel.

## Inspiration

OpenCode's commit command sends both `git diff --cached` (staged) and `git diff` (unstaged) to give the AI full context. Since yeet either stages everything or respects existing staging, `git diff --cached` always reflects what will be committed.

## Consequences

- Users can use yeet in both workflows: quick "commit everything" (`yeet`) and selective staging (`git add file && yeet`).
- Cancel is safe in both modes — manual staging is never lost.
- The AI always sees the correct diff (only what will be committed).
