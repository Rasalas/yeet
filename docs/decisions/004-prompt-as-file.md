# 004: System Prompt as Editable File

**Status:** Accepted
**Date:** 2025-02-27

## Context

The AI system prompt determines the format and style of generated commit messages. Users have different preferences (gitmoji, language, scope conventions, etc.). The prompt needs to be customizable.

## Decision

Store the prompt as a plain text file at `~/.config/yeet/prompt.txt`.

- Created automatically on first AI call with the default prompt.
- `yeet prompt` opens it in `$EDITOR`.
- `yeet prompt show` prints it.
- `yeet prompt reset` restores the default.

## Alternatives Considered

- **Field in config.toml**: Multi-line TOML strings are awkward to edit. A dedicated file is more natural.
- **Append-only custom rules**: A `extra_rules` field appended to a hardcoded base prompt. Safer but less flexible — users can't remove default rules they disagree with.
- **Both (prompt + extra_rules)**: Two config points for the same concern. Unnecessary complexity.

## Consequences

- Users have full control. They can break the prompt — `yeet prompt reset` recovers.
- The file is read on every AI call (no caching). This is fine since file reads are negligible compared to API latency.
- The default prompt is compiled into the binary as fallback.
