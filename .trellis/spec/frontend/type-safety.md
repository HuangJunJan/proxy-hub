# Type Safety

> Type safety patterns in this project.

---

## Overview

The frontend uses TypeScript with React. API payload types live in `web/src/lib/types.ts` and must match backend JSON contracts, including hyphenated config field names.

---

## Type Organization

### Scenario: Admin API and YAML-Compatible DTOs

#### 1. Scope / Trigger
- Trigger: frontend forms call admin APIs that mutate YAML-backed config.

#### 2. Signatures
- `OpenAIChannel`
- `APIKeyEntry`
- `ModelEntry`
- `ChannelsResponse`

#### 3. Contracts
- Use exact external JSON names: `"base-url"`, `"api-key-entries"`, `"timeout-sec"`.
- Do not silently translate these names in individual components.
- UI components can hold local form state with camelCase, but convert once in the API submit handler.

#### 4. Validation & Error Matrix
- Missing required config fields -> backend returns `{error}` and frontend displays it.
- Wrong DTO key name -> backend receives empty field and validation fails.

#### 5. Good/Base/Bad Cases
- Good: `api.createOpenAIChannel(channel)` receives an `OpenAIChannel`.
- Base: form state uses `{baseUrl, apiKey}` then maps to `{"base-url", "api-key-entries"}`.
- Bad: each page invents its own untyped payload object.

#### 6. Tests Required
- `pnpm build` must type-check DTOs.
- Future component tests should assert submitted JSON includes hyphenated names.

#### 7. Wrong vs Correct

Wrong:

```ts
body: JSON.stringify({ baseUrl, apiKeyEntries })
```

Correct:

```ts
const channel: OpenAIChannel = {
  name,
  "base-url": baseUrl,
  "api-key-entries": [{ "api-key": apiKey }],
  models,
};
```

Shared cross-page response types also belong in `web/src/lib/types.ts`; page-local UI state can stay local.

### Scenario: OpenAI Channel Model Mapping DTOs

#### 1. Scope / Trigger
- Trigger: channel forms and admin chat consume YAML-backed channel DTOs while backend routing supports OpenAI-compatible model pass-through.

#### 2. Signatures
- `OpenAIChannel.models?: ModelEntry[]`
- `OAuthChannel.models?: ModelEntry[]`
- `ModelEntry = { name: string; alias?: string }`
- `POST /api/admin/channels/probe-models -> { models: string[] }`
- `POST /api/admin/chat/completions` uses `AdminChatRequest.model`.

#### 3. Contracts
- `openai-api.models` is optional. Missing or empty means no explicit model enumeration; downstream requests still pass through with the requested model name unchanged.
- `models[]` entries are explicit enumeration or alias overrides only. `{ name }` advertises a 1:1 model in `/v1/models`; `{ name, alias }` rewrites downstream `alias` to upstream `name`.
- Probe results are candidates, not required config. The form may save zero selected rows.
- `chatgpt-oauth.models` may be absent in the admin API response, so UI must render it defensively, but backend validation still requires configured OAuth models.
- `GET /v1/models` displays only explicit `models[]` aliases/names; it cannot enumerate the infinite pass-through set.

#### 4. Validation & Error Matrix
- OpenAI channel with `models` omitted -> valid channel, default pass-through.
- Manual alias without an upstream model -> UI error before submit.
- Selected probed model with no alias -> save `{ name }`.
- Unknown admin chat model on an OpenAI channel -> backend passes the model through unchanged.

#### 5. Good/Base/Bad Cases
- Good: channel form saves `{ "models": [] }` or omits `models` for pass-through-only channels.
- Base: fetched rows default unselected; selected rows can leave alias blank.
- Bad: requiring at least one fetched model before allowing an OpenAI-compatible channel to save.

#### 6. Tests Required
- `pnpm build` must type-check optional `models` consumers.
- Backend config test asserts OpenAI channels without models validate.
- Backend router/server tests assert unknown OpenAI model names pass through unchanged.

#### 7. Wrong vs Correct

Wrong:

```ts
if (models.length === 0) throw new Error("model required");
```

Correct:

```ts
const models: ModelEntry[] = selectedRows.map((row) =>
  row.alias.trim() ? { name: row.name, alias: row.alias.trim() } : { name: row.name },
);
```

### Scenario: Admin Request Log DTOs

#### 1. Scope / Trigger
- Trigger: history and live-monitor pages consume persisted request logs and admin log search results.

#### 2. Signatures
- `GET /api/admin/logs?channel&apiKey&model&endpoint&requestType&statusClass&status&errorKind&from&to&limit&page`
- `RequestLog`
- `LogsResponse`

#### 3. Contracts
- Query fields are URL query strings; `from` and `to` are Unix milliseconds.
- `statusClass` is optional and uses `"success"` or `"error"`.
- `RequestLog` token fields are optional: `promptTokens`, `completionTokens`, `reasoningTokens`, and `totalTokens`.
- The history table displays token usage as input/output/reasoning/total.
- Cost estimation is not part of `RequestLog`; do not add cost columns without a backend pricing contract.

#### 4. Validation & Error Matrix
- Missing optional fields -> render `-`.
- Invalid date input -> omit the time parameter.
- Backend query failure -> page keeps its filter state and displays the backend error.

#### 5. Good/Base/Bad Cases
- Good: log filters build `URLSearchParams` and use `api.logs(params)`.
- Base: page-local filter state may be camelCase because it is not a backend DTO, but it must map once to query parameter names.
- Bad: table derives fake cost from token counts.

#### 6. Tests Required
- `pnpm build` must type-check `RequestLog` consumers.
- Store/admin backend tests must assert the query fields map to repository filters.
- Future UI tests should assert token labels render input/output/reasoning/total.

#### 7. Wrong vs Correct

Wrong:

```tsx
<DataTable headers={[t("token"), t("cost")]} rows={logs.map((log) => [log.totalTokens, "-"])} />
```

Correct:

```tsx
<TokenCell
  promptTokens={log.promptTokens}
  completionTokens={log.completionTokens}
  reasoningTokens={log.reasoningTokens}
  totalTokens={log.totalTokens}
/>
```

### Scenario: Admin Console Online Chat DTOs

#### 1. Scope / Trigger
- Trigger: authenticated console pages call an admin API that directly tests a selected upstream channel and model.

#### 2. Signatures
- `POST /api/admin/chat/completions`
- `AdminChatRequest`
- `AdminChatResponse`
- `ChatMessage`

#### 3. Contracts
- Request shape is `{ channelType: "openai-api", channelName: string, model: string, messages: ChatMessage[] }`.
- `model` is the requested downstream model. The backend resolves explicit aliases first; if no alias matches on an OpenAI-compatible channel, it forwards the requested model unchanged.
- `messages[]` contains `{ role: "system" | "user" | "assistant", content: string }`.
- Response shape is `{ content: string, promptTokens?: number, completionTokens?: number, totalTokens?: number, raw?: unknown }`.
- This endpoint uses admin session cookie auth, not downstream Bearer API keys.
- This endpoint is for selected-channel testing and currently supports only `openai-api`; it must not expose upstream API keys to the browser.

#### 4. Validation & Error Matrix
- Missing session -> HTTP 401 `{ error: "login required" }`.
- Unsupported `channelType` -> HTTP 501 `{ error }`.
- Missing `channelName`, `model`, or `messages` -> HTTP 400 `{ error }`.
- Channel not found -> HTTP 404 `{ error }`.
- Upstream non-2xx or read/decode failure -> HTTP 502 `{ error }`.

#### 5. Good/Base/Bad Cases
- Good: chat page builds channel/model options from `api.channels()` and sends `AdminChatRequest` through `api.chatCompletion`.
- Base: page-local chat history can stay in component state because it is not persisted configuration.
- Bad: browser calls an upstream provider directly with `api-key-entries[].api-key`.

#### 6. Tests Required
- Backend admin httptest must assert selected alias routes to the selected channel's real upstream model.
- `pnpm build` must type-check `AdminChatRequest` / `AdminChatResponse`.
- `go test ./...` must pass after endpoint changes.

#### 7. Wrong vs Correct

Wrong:

```ts
fetch(channel["base-url"], { headers: { Authorization: channel["api-key-entries"][0]["api-key"] } });
```

Correct:

```ts
await api.chatCompletion({
  channelName,
  channelType: "openai-api",
  model,
  messages,
});
```

---

## Validation

Runtime validation is currently server-owned. Frontend forms should keep lightweight required fields and display backend error text.

---

## Common Patterns

- Use `request<T>()` from `web/src/lib/api.ts` for typed API calls.
- Keep `EventSource` payload parsing at the live-log boundary and cast to `RequestLog` there.
- Use union types for persisted UI settings: `Language`, `ThemeMode`, `View`.

---

## Forbidden Patterns

- Do not use `any` for backend payloads.
- Do not duplicate config DTO shapes in page components.
- Do not define camelCase request DTOs for backend config endpoints unless an explicit mapper owns the conversion.
