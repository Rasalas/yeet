# yeet

Git commit & push in one command. With AI-generated commit messages.

```
$ yeet

  cmd/root.go              | 163 ++++++++++++++++++++++++++++++++++++++++++-----
  internal/ai/anthropic.go |  81 +++++++++++++++++++++++
  2 files changed, 225 insertions(+), 19 deletions(-)

  ⠋ Generating...
  > refactor(config): simplify provider initialization

  Enter commit  ·  e edit  ·  E $EDITOR  ·  Esc cancel

  [main 2dd3b34] refactor(config): simplify provider initialization
  Pushed to origin/main.
  $0.0001 · 3.1k in / 28 out · gpt-4o-mini
```

Commit messages stream in token-by-token with a spinner. Diff stats, messages, and costs are color-coded. Set `NO_COLOR=1` to disable colors.

## Install

```sh
go install github.com/rasalas/yeet@latest
```

Or build from source:

```sh
git clone https://github.com/rasalas/yeet.git
cd yeet
go build -o yeet .
```

## Usage

```sh
yeet                     # AI generates the commit message
yeet fix typo in readme  # Use your own message
yeet -m "config"         # -m flag for words that collide with subcommands
yeet -l                  # Local commit only (skip push)
```

**What happens:**

1. Checks for staged changes — if none, runs `git add --all`
2. Shows diff stat (insertions in green, deletions in red)
3. Generates commit message (AI with streaming) or uses your message
4. You review — Enter to commit, `e` to edit inline, `E` to open `$EDITOR`, Esc to cancel
5. `git commit` + `git push` (or only commit with `-l`)

Selective staging works too:

```sh
git add src/auth.go
yeet                     # Only commits auth.go
```

Pressing Escape cancels safely — if yeet auto-staged, it unstages. If you staged manually, your staging is preserved.

## Commands

| Command | Description |
|---------|-------------|
| `yeet [message...]` | Stage, commit, push |
| `yeet -l [message...]` | Stage, commit locally (no push) |
| `yeet config` | Full-screen TUI for provider/model/keys |
| `yeet config edit` | Open `config.toml` in `$EDITOR` |
| `yeet auth` | Show API key status |
| `yeet auth set <provider>` | Store API key in OS keyring |
| `yeet auth delete <provider>` | Remove API key from keyring |
| `yeet auth import [provider]` | Import keys from env vars / OpenCode into keyring |
| `yeet auth reset` | Remove all API keys from keyring |
| `yeet prompt` | Edit the AI system prompt in `$EDITOR` |
| `yeet prompt show` | Print the current prompt |
| `yeet prompt reset` | Reset prompt to default |
| `yeet eval plan` | Estimate eval sample + cost (no API calls) |
| `yeet eval generate` | Generate bounded A/B candidates from historical runs |
| `yeet eval judge` | Blind-judge A/B candidates (`A/B/tie/both bad`) |
| `yeet eval report` | Show win-rate, cost, latency, and phrase-hit stats |
| `yeet log` | Pretty `git log --oneline --graph` |

## Setup

### 1. Pick a provider

```sh
yeet config
```

Opens a TUI where you can select your AI provider, set models, and manage keys.

**Builtin providers:**

| Provider | Default model |
|----------|---------------|
| Anthropic | `claude-haiku-4-5-20251001` |
| OpenAI | `gpt-4o-mini` |
| Ollama (local) | `llama3` |

**Well-known providers** (OpenAI-compatible API):

| Provider | Default model |
|----------|---------------|
| Google | `gemini-3-flash-preview` |
| Groq | `llama-3.3-70b-versatile` |
| OpenRouter | `openrouter/auto` |
| Mistral | `mistral-small-latest` |

### 2. Set your API key

```sh
yeet auth set anthropic
```

Keys are stored in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service). Environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) are used as fallback.

If you have keys in environment variables or OpenCode's `auth.json`, import them into the keyring:

```sh
yeet auth import           # Import all available keys
yeet auth import groq      # Import a specific provider
```

## Auto provider

The default provider is `auto`. It picks the cheapest available provider by input token cost from all providers that have an API key configured. This means you can set up multiple providers and yeet will always use the most cost-effective one.

## Config

Config file: `~/.config/yeet/config.toml`

```toml
provider = "auto"

[anthropic]
model = "claude-haiku-4-5-20251001"

[openai]
model = "gpt-4o-mini"

[ollama]
model = "llama3"
url = "http://localhost:11434"
```

You can add custom providers that use the OpenAI Chat Completions format:

```toml
[custom.together]
model = "meta-llama/Llama-3-70b-chat-hf"
url = "https://api.together.xyz/v1"
env = "TOGETHER_API_KEY"
```

## AI Context

When generating a commit message, yeet sends the following to the AI:

- **Diff** — `git diff --cached` (truncated at 8000 lines)
- **File status** — `git status --short` for a quick overview of all changes
- **Branch name** — used as hint for commit type and scope
- **Recent commits** — `git log --oneline -10` so the AI matches your style

## Custom Prompt

The system prompt lives at `~/.config/yeet/prompt.txt` and is created automatically on first run.

```sh
yeet prompt         # Edit in $EDITOR
yeet prompt show    # View current prompt
yeet prompt reset   # Restore default
```

## Eval (separate from commit flow)

`yeet eval` is an explicit, opt-in workflow for comparing prompt/model variants on real historical runs.

- Normal `yeet` commits stay fast — no extra model calls during commit.
- Run data is stored locally in SQLite: `~/.local/share/yeet/yeet.db`
- No upload/telemetry by default.
- Requires `sqlite3` to be available on `PATH` for eval commands/storage.
- More detail: [`docs/eval.md`](docs/eval.md)

Typical flow:

```sh
yeet eval plan --sample 20 --batch-size 5 --max-cost-usd 1.00
yeet eval generate --sample 20 --batch-size 5 --max-cost-usd 1.00 --prompt-file ./prompt-v2.txt
yeet eval judge --variant 3
yeet eval report --variant 3
```

Useful filters:

```sh
# Evaluate only runs where baseline output contains a phrase you want to remove
yeet eval generate --contains "for improved user" --sample 30 --batch-size 8

# Focus on runs where you had to edit the AI output
yeet eval generate --edited-only --sample 30 --batch-size 8
```
