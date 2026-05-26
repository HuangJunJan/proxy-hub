# ChatGPT OAuth / Codex Upstream Spike

## Date

2026-05-26

## Findings

- Public ChatGPT OAuth proxy implementations route Codex-style traffic through ChatGPT backend Codex endpoints rather than the standard OpenAI Chat Completions API.
- The relevant endpoint shape is commonly implemented around `https://chatgpt.com/backend-api/codex/responses`.
- The adapter is not a thin base URL + bearer token forwarder. It needs:
  - OAuth token refresh.
  - Codex endpoint request construction.
  - Translation between OpenAI-compatible `chat/completions` payloads and Codex response payloads.
  - Stream event translation back into OpenAI-compatible SSE chunks.
- ChatGPT OAuth model listing is not a standard `GET /v1/models` flow. The v1 PRD already keeps those models manually configured with optional presets.

## Implementation Impact

- Implement `openai-api` first as the v1 stable adapter path.
- Keep the `chatgpt-oauth` config shape and router contract intact.
- Treat the actual ChatGPT OAuth adapter as a separate implementation block after a dedicated translator design is written.
- If the translator work exceeds the remaining v1 budget, mark ChatGPT OAuth execution as v1.1 while preserving the config and UI affordances.

## Sources Consulted

- Existing public ChatGPT OAuth proxy patterns and Codex endpoint naming from current GitHub-searchable implementations.
- Current task PRD/design/implement artifacts under `.trellis/tasks/05-25-proxy-hub/`.
