# PRD: Frontend API Hooks State Cleanup

## Goal
Reduce repeated request/loading/error boilerplate by introducing shared frontend hooks where useful while preserving centralized API access and global error handling.

## Requirements
1. API calls must still go through `web/src/lib/api.ts`.
2. Generic API errors remain global; no page-local generic error toasts.
3. Add reusable hooks only when they reduce duplication without hiding page-specific behavior.
4. Preserve all current user flows.

## Acceptance Criteria
- `pnpm --dir web build` passes.
- At least one reusable hook reduces repeated async request boilerplate.
- No API contract changes.
