# PRD: Frontend Page Refactor

## Goal
Reduce route-page complexity by extracting reusable page sections into feature components while preserving existing behavior and API contracts.

## Requirements
1. Keep route files under `web/src/pages/*-page.tsx` as orchestration layers.
2. Move repeated or bulky domain UI into `web/src/features/*`.
3. Preserve all existing routes and user flows.
4. Keep API calls through `web/src/lib/api.ts`.
5. Do not reintroduce page-local generic API error toasts.

## Acceptance Criteria
- `pnpm --dir web build` passes.
- Logs, keys, dashboard, chat, and channels pages remain functional.
- At least the largest reusable page sections are extracted from route files.
- Setup token visibility remains unchanged.
