# 003: AI Context — What Gets Sent to the Model

**Status:** Accepted
**Date:** 2025-02-27

## Context

The quality of AI-generated commit messages depends on what context the model receives. Too little context produces generic messages. Too much wastes tokens and adds latency.

## Decision

Send three pieces of information:

1. **`git diff --cached`** — The actual changes. Truncated at 8000 lines.
2. **`git log --oneline -10`** — Recent commits so the model matches the repo's commit style and language.
3. **Branch name** — `feat/user-auth` is a strong hint for type (`feat`) and scope (`auth`).

All three are included in the user message. The system prompt instructs the model to use branch name and recent commits when available.

## Research

| Tool | Diff | Recent Commits | Branch | File Status |
|------|------|----------------|--------|-------------|
| aicommits (Nutlope) | ✓ | — | — | — |
| Gemini CLI (Google) | ✓ | ✓ | — | ✓ |
| gcm.js (cebre) | ✓ | — | — | — |
| **yeet** | ✓ | ✓ | ✓ | — |

Most tools send only the diff. Google's Gemini CLI goes furthest with `git status` + `git log`. We follow that pattern and add the branch name.

## Consequences

- Branch and log are fetched gracefully — errors are silently ignored (e.g., fresh repo with no commits).
- The additional context is small (~10 lines for log + 1 line for branch) and doesn't meaningfully increase token usage.
- Commit style consistency improves significantly in repos with established conventions.
