# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Proxy Hub backend code is a Go service with these core boundaries:

- YAML config is the source of truth for operator-managed data.
- SQLite stores runtime logs and aggregate statistics only.
- `/v1/*` responses must be OpenAI-compatible.
- Streaming upstream responses must remain alive until the proxy finishes relaying the body.

---

## Forbidden Patterns

### Do Not Cancel Upstream Stream Contexts Before Body Relay

#### 1. Scope / Trigger
- Trigger: upstream adapters return `io.ReadCloser` bodies to proxy handlers.

#### 2. Signatures
- `upstream.Adapter.Chat(ctx context.Context, req upstream.ChatRequest) (*upstream.ChatResponse, error)`
- `upstream.ChatResponse.Body io.ReadCloser`

#### 3. Contracts
- Adapter may create a timeout context for the upstream HTTP request.
- Adapter must not `defer cancel()` before returning a response body.
- Cancellation must happen when the returned body is closed by the proxy layer.

#### 4. Validation & Error Matrix
- Upstream request creation fails -> cancel immediately and return wrapped error.
- Upstream `Do` fails -> cancel immediately and return wrapped error.
- Upstream returns a response -> wrap body so `Close()` cancels the request context.

#### 5. Good/Base/Bad Cases
- Good: streaming response relays chunks until EOF or client cancel.
- Base: non-streaming response is read, body closed, context canceled.
- Bad: `defer cancel()` in adapter causes stream reads to fail after return.

#### 6. Tests Required
- Adapter stream test must read the returned body after `Chat` returns.
- Proxy/server integration tests must cover response relay after adapter return.

#### 7. Wrong vs Correct

Wrong:

```go
ctx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
resp, err := client.Do(req)
return &upstream.ChatResponse{Body: resp.Body}, err
```

Correct:

```go
ctx, cancel := context.WithTimeout(ctx, timeout)
resp, err := client.Do(req)
if err != nil {
    cancel()
    return nil, err
}
return &upstream.ChatResponse{Body: cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}}, nil
```

### Do Not Re-Define Config Field Names Per Layer

#### 1. Scope / Trigger
- Trigger: config structs are used across YAML, admin JSON API, router, scheduler, and frontend forms.

#### 2. Signatures
- `config.Config`
- `config.OpenAIAPIChannel`
- `config.APIKeyEntry`
- `config.ModelEntry`

#### 3. Contracts
- YAML tags and JSON tags must describe the same external field name.
- Hyphenated fields such as `base-url`, `api-key-entries`, and `timeout-sec` must be accepted by admin APIs.
- Frontend request types must match those external names.

#### 4. Validation & Error Matrix
- Missing `base-url` -> config validation error.
- Missing `api-key-entries` -> config validation error.
- Mismatched JSON tag -> admin API receives an empty field and incorrectly fails validation.

#### 5. Good/Base/Bad Cases
- Good: one struct owns YAML and JSON boundary fields.
- Base: internal code calls `Effective*` helpers for defaulted values.
- Bad: frontend sends `base-url` while Go only accepts `BaseURL`.

#### 6. Tests Required
- Admin create/update channel tests must post JSON with YAML-compatible hyphenated field names.
- Config tests must verify default normalization and validation.

#### 7. Wrong vs Correct

Wrong:

```go
BaseURL string `yaml:"base-url,omitempty"`
```

Correct:

```go
BaseURL string `yaml:"base-url,omitempty" json:"base-url,omitempty"`
```

### Do Not Log Secrets

- Never log upstream `api-key`, downstream bearer tokens, OAuth access tokens, or refresh tokens.
- Request logs use `auth.MaskToken(token)` for downstream API keys.
- Upstream keys may only appear as key index metadata, not plaintext.
- Authenticated admin config APIs may return downstream API key plaintext for copy/edit UX because YAML is the configuration source of truth and stores these keys in plaintext. This exception does not apply to logs, metrics, proxy responses, or unauthenticated endpoints.

---

## Required Patterns

### OpenAI-Compatible API Errors

#### 1. Scope / Trigger
- Trigger: any `/v1/*` endpoint error response.

#### 2. Signatures
- `auth.AbortOpenAIError(c, status, message, type, code)`

#### 3. Contracts
- Error body shape must be `{ "error": { "message", "type", "code" } }`.
- Invalid downstream key must return HTTP 401 with `code: "invalid_api_key"`.
- Missing model must return HTTP 404 with `code: "model_not_found"`.

#### 4. Validation & Error Matrix
- Invalid bearer token -> 401 `invalid_api_key`.
- Unknown model alias -> 404 `model_not_found`.
- No channel available -> 503 `no_available_channel`.
- Upstream 401/403 -> 500 `auth_failed` without leaking credentials.

#### 5. Good/Base/Bad Cases
- Good: OpenAI SDK clients can parse every `/v1/*` error.
- Base: admin APIs may return simple `{error}` JSON.
- Bad: `/v1/*` returns Gin default plain text or admin-style errors.

#### 6. Tests Required
- Server tests must decode `/v1/*` error body and assert `error.code`.

#### 7. Wrong vs Correct

Wrong:

```go
c.JSON(http.StatusUnauthorized, gin.H{"error": "bad key"})
```

Correct:

```go
auth.AbortOpenAIError(c, http.StatusUnauthorized, "Invalid API key provided.", "invalid_request_error", "invalid_api_key")
```

### OpenAI-Compatible Root Routes for BYOK Clients

#### 1. Scope / Trigger
- Trigger: registering OpenAI-compatible proxy routes in `internal/server`.

#### 2. Signatures
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `GET /models`
- `POST /chat/completions`
- `POST /responses`

#### 3. Contracts
- `/v1/*` remains the documented OpenAI base URL contract.
- Root routes exist as BYOK compatibility aliases for clients that treat the configured base URL as the origin and append `/chat/completions`, `/responses`, or `/models`.
- Both route sets must use the same proxy handler instance, bearer auth middleware, router index, scheduler, and monitor submission behavior.
- Both route sets must require the same `Authorization: Bearer <proxy-hub-api-key>` header.
- `POST /v1/responses` and `POST /responses` are transparent OpenAI-compatible Responses API pass-through routes: parse the top-level `model` and optional `stream`, apply alias routing/model replacement, then relay the upstream body without converting it to Chat Completions.

#### 4. Validation & Error Matrix
- Missing or invalid bearer token on either route set -> HTTP 401 OpenAI-compatible `invalid_api_key`.
- Unknown model on either chat or responses route -> HTTP 404 OpenAI-compatible `model_not_found`.
- Unknown API route outside these aliases -> HTTP 404 admin-style `{ "error": "not found" }`.

#### 5. Good/Base/Bad Cases
- Good: BYOK client configured with `http://localhost:8787` can call `/chat/completions` or `/responses`.
- Base: OpenAI SDK configured with `http://localhost:8787/v1` can call `/v1/chat/completions` or `/v1/responses`.
- Bad: root `/responses` falls through to SPA/API fallback and returns generic `{"error":"not found"}`.

#### 6. Tests Required
- Server httptest for `/v1/models` requiring bearer auth.
- Server httptest for `/v1/chat/completions` alias routing to upstream model.
- Regression test for `/chat/completions` with a configured model alias returning the same successful upstream response.
- Server httptest for `/v1/responses` alias routing to upstream model.
- Regression test for `/responses` with a configured model alias returning the same successful upstream response.

#### 7. Wrong vs Correct

Wrong:

```go
v1 := r.Group("/v1")
v1.Use(requireBearer(opts.ConfigManager))
proxy.NewHandler(opts.ConfigManager, nil, opts.Monitor, opts.Logger).Register(v1)
```

Correct:

```go
proxyHandler := proxy.NewHandler(opts.ConfigManager, nil, opts.Monitor, opts.Logger)
v1 := r.Group("/v1")
v1.Use(requireBearer(opts.ConfigManager))
proxyHandler.Register(v1)

openAICompat := r.Group("")
openAICompat.Use(requireBearer(opts.ConfigManager))
proxyHandler.Register(openAICompat)
```

### Preserve Last-Mile Observability

- Proxy handlers submit request logs after response relay when possible so duration covers actual relay time.
- Non-streaming successful responses may be buffered to parse `usage` and apply body logging policy.
- Streaming successful responses are not buffered; token usage parsing requires a dedicated stream parser before enabling stream token totals.

### OpenAI Channel Model Pass-Through

#### 1. Scope / Trigger
- Trigger: config validation, router indexing, proxy handlers, admin chat, and frontend channel forms all share the channel model contract.

#### 2. Signatures
- `config.OpenAIAPIChannel.Models []config.ModelEntry`
- `config.ChatGPTOAuthChannel.Models []config.ModelEntry`
- `router.NewIndex(cfg *config.Config) *Index`
- `(*router.Index).Resolve(model string) []router.Hit`
- `(*router.Index).Models() []string`

#### 3. Contracts
- `openai-api.models` is optional. Empty means all requested downstream model names are eligible for pass-through on enabled OpenAI-compatible channels.
- Explicit `models[]` entries still win first: alias/name matches route to the configured upstream `ModelEntry.Name`.
- When no explicit model matches, `Resolve(requested)` returns enabled OpenAI-compatible hits with `Hit.UpstreamModelName = requested`.
- `chatgpt-oauth.models` remains required and does not get pass-through fallback.
- `Index.Models()` returns only explicit aliases/names from `models[]`; it must not attempt to enumerate pass-through models.
- Admin selected-channel chat follows the same OpenAI pass-through rule: empty requested model is invalid, unknown non-empty model is forwarded unchanged for OpenAI-compatible channels.

#### 4. Validation & Error Matrix
- OpenAI channel with empty models -> validation succeeds.
- OAuth channel with empty models -> validation error on `chatgpt-oauth[n].models`.
- Explicit alias match -> upstream model is the configured `name`.
- No explicit match + enabled OpenAI channel -> upstream model is the requested model.
- No explicit match + no enabled OpenAI channel -> `/v1/*` returns OpenAI-compatible `model_not_found`.

#### 5. Good/Base/Bad Cases
- Good: `Resolve("gpt-4.1")` returns pass-through hits for an enabled OpenAI channel with no models.
- Base: `GET /v1/models` returns configured aliases only, even though pass-through accepts more names.
- Bad: treating `models[]` as an allowlist and returning 404 for every unlisted OpenAI model.

#### 6. Tests Required
- `internal/config` tests for OpenAI optional models and OAuth required models.
- `internal/router` tests for alias priority and OpenAI pass-through fallback.
- `internal/server` tests for chat/completions and responses pass-through, plus model_not_found when no OpenAI fallback exists.

#### 7. Wrong vs Correct

Wrong:

```go
hits := idx.aliasToHits[normalizeAlias(model)]
return append([]Hit(nil), hits...)
```

Correct:

```go
if hits := idx.aliasToHits[normalizeAlias(model)]; len(hits) > 0 {
	return cloneHits(hits)
}
return idx.passthroughHits(model)
```

---

## Testing Requirements

Backend changes need tests at the lowest layer that owns the behavior plus one integration test when behavior crosses layers.

- Config schema/default behavior -> `internal/config` tests.
- Routing and scheduler behavior -> `internal/router` / `internal/scheduler` tests.
- `/v1/*` auth, alias routing, failover, and OpenAI error shape -> `internal/server` httptests.
- Request logging/statistics -> `internal/monitor` plus one proxy/server integration test.
- Upstream adapters -> `internal/upstream/<provider>` httptests.

---

## Code Review Checklist

- [ ] `/v1/*` error responses use OpenAI-compatible body shape.
- [ ] Any returned stream body owns cancellation until `Close()`.
- [ ] Config structs have matching YAML and JSON tags for external fields.
- [ ] Default values are read via `Effective*` helpers and normalized before save.
- [ ] Request logs do not include plaintext secrets.
- [ ] Cross-layer changes have both layer-level tests and an integration test.
