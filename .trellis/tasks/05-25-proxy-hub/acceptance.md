# Acceptance Status

Date: 2026-05-26

## Validation Commands Run

- `go test ./...` - passed
- `go build ./...` - passed
- `pnpm build` from `web/` - passed, including the shadcn/Radix sheet refactor and layered frontend structure
- `go build -ldflags="-s -w" -o dist/proxy-hub.exe ./cmd/proxy-hub` - passed

## Covered In This Implementation Pass

- AC-2 partial: `POST /v1/chat/completions` is implemented for `openai-api`; non-stream and stream bodies are relayed. Stream behavior is covered by adapter tests, not a full end-to-end curl.
- AC-3 / AC-4 covered by httptest for OpenAI-compatible channels: a high-priority upstream returning 401 is skipped, the next channel succeeds, attempts are logged, and the failing channel is circuit-opened.
- AC-5 covered for upstreams that return OpenAI `usage`: channel request/success/failure/latency aggregation exists, token totals are parsed from non-stream responses, persisted, aggregated, and displayed by the dashboard.
- AC-6 partial: realtime SSE stream exists and frontend live page subscribes. Channel/status filtering is implemented for historical logs, not the live page.
- AC-7 covered: theme mode is light/dark/system and persists in `localStorage`.
- AC-8 partial: console has zh/en dictionary coverage for current UI text. It is a lightweight in-file dictionary, not a full i18next file set.
- FR-8.1 covered for the implemented console slice: the frontend uses React + Vite + Axios, shadcn-style source components under `web/src/components/ui`, Tailwind CSS 4, Radix primitives for dialog/sheet/tabs, and layered `pages` / `features` / `components` / `lib` structure.
- AC-9 covered for implemented channel/key flows: admin API writes YAML through the atomic config manager and updates the in-memory snapshot.
- AC-10 partial: YAML config and SQLite request/stat data persist across restarts. End-to-end restart verification was not manually run.
- AC-11 partial: `dist/proxy-hub.exe` was built, and the main process attempts to open the browser unless `--no-browser` is set. Double-click packaged exe was not manually tested.
- AC-13 partial: frontend and admin API can fetch upstream OpenAI-compatible models and show chips. Selection UX is minimal; manual model/alias entry works.
- AC-14 covered by httptest: downstream `gpt-5.4` routes to upstream `deepseek-chat`.
- AC-15 covered by httptest: `GET /v1/models` returns enabled aliases.
- AC-16 covered: missing config enters setup mode; setup writes admin + first API key and displays the token before entering the console.
- AC-17 covered at scheduler level: multi-key round-robin is tested and the selected upstream key index is stored on request logs.
- AC-18 covered by config tests: default values normalize out before YAML save.
- FR-7.3 covered: request log retention cleanup is wired into the monitor service and covered by unit test.
- FR-7.4 covered for non-stream requests: `failed_only` / `always` / `none` body-mode logic is implemented; `always` is covered by proxy-level httptest.

## Not Yet Complete

- AC-1: manual health-check flow exists for OpenAI-compatible channels, but there is no end-to-end manual run with two real upstreams.
- AC-5 token totals for streaming responses and upstreams that omit `usage`: proxy does not yet parse stream chunks or estimate tokens.
- AC-6 live filtering: live page currently shows recent stream entries without channel/status filtering controls.
- AC-10 restart verification: persistence is implemented but not manually verified with a restart run.
- AC-11 exe double-click packaging: `dist/proxy-hub.exe` was produced in this validation pass, but double-click launch/browser behavior was not manually tested.
- AC-12 / FR-10: `chatgpt-oauth` config shape and route indexing exist, but actual OAuth refresh + Codex/ChatGPT translator adapter is not implemented. See `research/chatgpt-oauth-codex-spike.md`.
- FR-9.3: `/metrics` is reserved in design only; no endpoint yet.

## Notes

The implemented vertical slice is usable for OpenAI-compatible upstreams and provides the intended YAML + SQLite + embedded React control-plane foundation. The largest remaining product risk is ChatGPT OAuth because it requires a non-standard Codex/ChatGPT backend translator rather than simple OpenAI-compatible forwarding.
