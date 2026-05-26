# Error Handling

> How errors are handled in this project.

---

## Overview

Proxy Hub has two public error surfaces:

- `/v1/*` OpenAI-compatible API errors for downstream clients.
- `/api/admin/*` simple JSON errors for the control console.

---

## Error Types

- `/v1/*`: use `auth.OpenAIErrorBody`.
- Admin API: use `{"error": "<message>"}` and HTTP status codes.
- Internal admin mutator errors may use small typed errors with status and message.

---

## Error Handling Patterns

### Scenario: Upstream Failure Mapping

#### 1. Scope / Trigger
- Trigger: OpenAI-compatible proxy attempts multiple upstream selections.

#### 2. Signatures
- `classifyFailure(upstreamFailure) (status int, typ string, code string)`
- `auth.AbortOpenAIError(c, status, message, typ, code)`

#### 3. Contracts
- Upstream 401/403 maps to downstream HTTP 500 `auth_failed` so upstream credential details are not exposed.
- Upstream 429 maps to downstream HTTP 429 `rate_limit_exceeded`.
- Upstream 5xx maps to downstream HTTP 502 `bad_gateway`.
- Timeout maps to downstream HTTP 504 `gateway_timeout`.
- Unknown model maps before upstream calls to HTTP 404 `model_not_found`.

#### 4. Validation & Error Matrix
- First upstream fails and second succeeds -> return success and log attempts = 2.
- All upstreams fail -> return last normalized failure.
- Channel circuit open -> skip channel and attempt next selection.

#### 5. Good/Base/Bad Cases
- Good: users see a sanitized OpenAI-compatible error.
- Base: logs retain error kind and summary for the admin console.
- Bad: response body includes upstream API key, raw auth error, or non-OpenAI shape.

#### 6. Tests Required
- Server httptest where first upstream returns 401 and second returns 200.
- Assert response status 200, first upstream called once, second called, and log attempts = 2.

#### 7. Wrong vs Correct

Wrong:

```go
c.String(resp.StatusCode, rawUpstreamBody)
```

Correct:

```go
status, typ, code := classifyFailure(last)
auth.AbortOpenAIError(c, status, message, typ, code)
```

---

## API Error Responses

### OpenAI-Compatible Shape

```json
{
  "error": {
    "message": "Invalid API key provided.",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

### Admin Shape

```json
{
  "error": "channel not found"
}
```

---

## Common Mistakes

### Common Mistake: Treating Admin and Downstream Errors as the Same Contract

**Symptom**: OpenAI SDK clients cannot parse Proxy Hub errors.

**Cause**: A `/v1/*` handler returned admin-style `{error: "..."}` JSON.

**Fix**: Use `auth.AbortOpenAIError` for all `/v1/*` errors.

**Prevention**: Add/keep httptests that decode `auth.OpenAIErrorBody` for `/v1/*` failure cases.
