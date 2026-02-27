# 005: API Keys in OS Keyring, Not Config

**Status:** Accepted
**Date:** 2025-02-27

## Context

AI providers require API keys. These need to be stored somewhere accessible to the CLI.

## Decision

API keys are stored in the OS keyring via `go-keyring`:

- **macOS**: Keychain
- **Linux**: Secret Service (GNOME Keyring / KWallet)
- **Windows**: Credential Manager

Environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) serve as fallback when the keyring is empty.

Keys never appear in config files.

## Alternatives Considered

- **In config.toml**: Simple but insecure. Config files end up in dotfile repos, backups, and screenshots.
- **Separate .env file**: Better than config.toml but still a plaintext file on disk. Easy to accidentally commit.
- **Only environment variables**: Secure but inconvenient. Requires shell config and is per-session on some setups.

## Consequences

- `go-keyring` adds a CGO dependency on Linux (for D-Bus/Secret Service). On macOS and Windows it uses native APIs without CGO.
- Ollama doesn't need an API key, so keyring is skipped for it.
- `yeet auth` provides a clear status view of which keys are configured.
