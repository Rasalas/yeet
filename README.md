# yeet

Git commit & push in one command. With AI-generated commit messages.

```
$ yeet

  main.go    | 12 ++++++----
  config.go  |  3 ++-
  2 files changed, 9 insertions(+), 6 deletions(-)

  > refactor(config): simplify provider initialization

  Enter commit  ·  e edit  ·  E $EDITOR  ·  Esc cancel

  [main 2dd3b34] refactor(config): simplify provider initialization
  Pushed to origin/main.
```

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
```

**What happens:**

1. Checks for staged changes — if none, runs `git add --all`
2. Shows diff stat
3. Generates commit message (AI) or uses your message
4. You review — Enter to commit, `e` to edit inline, `E` to open `$EDITOR`, Esc to cancel
5. `git commit` + `git push`

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
| `yeet config` | Full-screen TUI for provider/model/keys |
| `yeet auth` | Show API key status |
| `yeet auth set <provider>` | Store API key in OS keyring |
| `yeet auth delete <provider>` | Remove API key from keyring |
| `yeet prompt` | Edit the AI system prompt in `$EDITOR` |
| `yeet prompt show` | Print the current prompt |
| `yeet prompt reset` | Reset prompt to default |
| `yeet log` | Pretty `git log --oneline --graph` |

## Setup

### 1. Pick a provider

```sh
yeet config
```

Opens a TUI where you can select your AI provider, set models, and manage keys.

Supported providers:

- **Anthropic Claude** (default) — `claude-sonnet-4-20250514`
- **OpenAI** — `gpt-4o`
- **Ollama** (local) — `llama3`

### 2. Set your API key

```sh
yeet auth set anthropic
```

Keys are stored in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service). Environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) are used as fallback.

## Config

Config file: `~/.config/yeet/config.toml`

```toml
provider = "anthropic"

[anthropic]
model = "claude-sonnet-4-20250514"

[openai]
model = "gpt-4o"

[ollama]
model = "llama3"
url = "http://localhost:11434"
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