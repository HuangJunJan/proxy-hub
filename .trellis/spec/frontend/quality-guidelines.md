# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

The frontend is an operational console, not a marketing page. It prioritizes dense data views, predictable controls, and direct interaction with backend state.

---

## Forbidden Patterns

- Do not use one-off untyped fetch calls from page components; use `web/src/lib/api.ts`.
- Do not hide setup-created API keys by navigating away immediately. The first token must remain visible until the user chooses to continue.

---

## Required Patterns

### Scenario: Setup Token Visibility and Persistent UI Preferences

#### 1. Scope / Trigger
- Trigger: first-run setup and console settings span backend session, YAML write, and local browser state.

#### 2. Signatures
- `POST /api/admin/setup -> { token: string }`
- `GET /api/admin/keys -> Array<{ name?: string, notes?: string, token: string, tokenMask: string, disabled?: boolean }>`
- `ThemeMode = "light" | "dark" | "system"`
- `Language = "zh" | "en"`

#### 3. Contracts
- Setup must display the first generated API key before entering the console.
- API Keys page may display/copy full downstream tokens for the authenticated admin console; `tokenMask` remains available for compact display and request logs.
- Theme and language changes persist in `localStorage`.
- Theme is applied to `document.documentElement.dataset.theme`.

#### 4. Validation & Error Matrix
- Setup API succeeds -> token is displayed and session cookie is set.
- Setup API fails -> page remains on setup and displays backend error.
- Keys list API succeeds -> copy action copies `token`, not `tokenMask`.
- Theme/language change -> value persists after refresh.

#### 5. Good/Base/Bad Cases
- Good: setup form shows token in a code block and a separate continue action.
- Base: settings page exposes language and theme selectors; API Keys list exposes copy/edit/enable-disable actions.
- Bad: setup calls `onDone` immediately after receiving token.

#### 6. Tests Required
- `pnpm build` must pass.
- Backend admin tests must assert `GET /api/admin/keys` includes `token` for authenticated admin copy actions.
- Future UI tests should assert setup token remains visible after setup success.

#### 7. Wrong vs Correct

Wrong:

```tsx
const result = await api.setup(username, password);
setToken(result.token);
onDone(username);
```

Correct:

```tsx
const result = await api.setup(username, password);
setToken(result.token);
// User clicks continue after seeing the token.
```

---

## Testing Requirements

- `pnpm build` is the required type/build gate.
- Frontend changes touching API contracts must be paired with backend httptests or API client type changes.

---

## Code Review Checklist

- [ ] API calls go through `web/src/lib/api.ts`.
- [ ] DTO fields match backend JSON names.
- [ ] Setup-created token remains visible before entering the console.
- [ ] API Keys copy buttons copy full tokens when the admin keys API returns `token`.
- [ ] Theme and language state persists through `localStorage`.
- [ ] Operational tables fit on mobile via horizontal scroll rather than overlapping text.
