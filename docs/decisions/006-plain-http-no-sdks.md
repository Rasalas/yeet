# 006: Plain net/http for AI Providers, No SDKs

**Status:** Accepted
**Date:** 2025-02-27

## Context

yeet integrates with three AI providers (Anthropic, OpenAI, Ollama). Each offers official Go SDKs.

## Decision

All providers use plain `net/http` + `encoding/json`. No SDKs.

Each provider implementation is ~80 lines: build request, POST, parse response, return message.

## Rationale

- **Minimal dependency tree**: SDKs pull in their own dependencies, version constraints, and abstractions. The actual API surface we use is one endpoint per provider.
- **Uniform error handling**: All three providers follow the same pattern — HTTP POST with JSON body, JSON response with error field.
- **Transparency**: The full request/response is visible in the code. No hidden retry logic, no middleware, no auth wrappers.
- **Stability**: HTTP APIs are stable. SDKs release breaking changes, rename types, deprecate methods. `net/http` doesn't change.

## Consequences

- We handle API version headers manually (e.g., `anthropic-version: 2023-06-01`).
- No automatic retries or rate limit handling. Acceptable for a single-request CLI tool.
- Adding a new provider means ~80 lines of boilerplate. This is a feature, not a bug — each provider is self-contained.
