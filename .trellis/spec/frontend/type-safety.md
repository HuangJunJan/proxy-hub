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
