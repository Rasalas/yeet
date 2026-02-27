# 008: Color Scheme — "Hot Mango"

**Status:** Accepted
**Date:** 2026-02-27

## Context

yeet's terminal UI uses color in two places:

1. **Main flow** (`cmd/root.go`) — ANSI escape codes for the commit message display box, diff stat coloring, and status indicators.
2. **Config TUI** (`internal/tui/tui.go`) — Lipgloss styles for the interactive provider/model picker.

The original palette was purple-based (`#7C3AED` primary), which felt generic and didn't match yeet's personality as a fast, fun, action-oriented tool.

## Decision

Adopt the **"Hot Mango"** palette — warm, energetic, and immediately recognizable:

| Role      | Hex       | Usage                              |
|-----------|-----------|------------------------------------|
| Primary   | `#FF8C42` | Titles, selected items, accent bar |
| Secondary | `#FFB074` | Labels, highlights                 |
| Muted     | `#78716C` | Help text, separators, bullets     |
| Success   | `#4ADE80` | Checkmarks, confirmations          |
| Danger    | `#FF6B6B` | Errors, missing keys               |
| Warning   | `#FBBF24` | Custom/non-default model indicator |
| Text      | `#FEF9F3` | Normal text (cream white)          |

The commit message box uses a warm dark background (`rgb(44, 36, 30)`) with a mango-colored left bar.

Standard ANSI green/red (`\033[32m` / `\033[31m`) are kept for diff stat `+`/`-` coloring, as these are universally understood.

All colors respect `NO_COLOR` — when the environment variable is set, output falls back to plain text.

## Rationale

- **Orange is action.** It matches yeet's "throw it out there" energy better than cool purple.
- **Warm tones are distinctive.** Few CLI tools use this range, making yeet visually memorable.
- **Palette cohesion.** All colors share warm undertones (coral red, sunny yellow, cream white) rather than mixing warm and cool.
- **Functional colors stay functional.** Diff stat green/red use standard ANSI codes for maximum terminal compatibility.

## Where colors are defined

- **ANSI escapes:** `cmd/root.go` — `msgBar`, `msgBg`, `msgOpen`, `msgClose` variables
- **Lipgloss colors:** `internal/tui/tui.go` — `colorPrimary` through `colorText` variables
