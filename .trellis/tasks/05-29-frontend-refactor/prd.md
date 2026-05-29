# PRD: Frontend Project Refactor

## Goal
Refactor the entire frontend admin console with a phased full-scope approach: improve visual consistency, align the UI with a shadcn-style admin dashboard, clean up frontend architecture, and preserve existing business behavior.

## User Value
- The console should feel coherent, professional, and easier to operate.
- Future feature work should be easier because pages, components, API access, and styling boundaries are clearer.
- Error handling should remain centralized through the global axios interceptor and global toast.

## Confirmed Facts
- The frontend lives under `web/src` and uses React, Vite, TypeScript, axios, Radix primitives, and local shadcn-style source components.
- The project has `web/components.json` with shadcn aliases configured.
- UI primitives live under `web/src/components/ui`.
- API calls are centralized through `web/src/lib/api.ts`.
- Global axios error interception and global toast were added before this task.
- Current page components contain a mix of route orchestration, API calls, form state, business logic, and markup.
- Current visual styling is mostly custom CSS in `web/src/styles.css`, not standard shadcn class styling.
- The user chose option C: visual + architecture refactor, implemented in phases.
- The user approved splitting the refactor into parent + child tasks.

## Child Task Map
1. `.trellis/tasks/05-29-frontend-ui-foundation` — establish UI foundation, tokens, primitives, and global visual language.
2. `.trellis/tasks/05-29-frontend-app-shell` — refactor app shell, navigation, topbar, route layout, and responsive behavior.
3. `.trellis/tasks/05-29-frontend-page-refactor` — refactor route pages and domain feature sections.
4. `.trellis/tasks/05-29-frontend-api-hooks-state` — extract API hooks/state helpers and reduce repeated request state logic.
5. `.trellis/tasks/05-29-frontend-css-cleanup` — remove stale CSS and consolidate final style rules after feature refactors.

## Requirements
1. Preserve current backend API contracts and current admin console business capabilities.
2. Use shadcn-style local source components as the primary UI foundation.
3. Improve visual consistency across app shell, dashboard, channel management, chat, keys, logs, setup, login, and settings.
4. Reduce route-page complexity by moving reusable domain sections, hooks, helpers, or form components into appropriate `features/`, `components/`, or `lib/` modules.
5. Keep API access through `web/src/lib/api.ts`; do not add page-local fetch/axios calls.
6. Keep global error handling centralized; do not reintroduce per-page generic error toasts.
7. Keep setup-created token visible until the user chooses to continue.
8. Keep theme and language behavior intact.
9. Keep DTO names compatible with backend JSON contracts, including hyphenated config fields.
10. Validate with the frontend build after each major phase.
11. Execute phases in child-task order unless implementation findings require replanning.

## Acceptance Criteria
- `pnpm --dir web build` passes at parent integration completion.
- No page-local generic error state/toast remains for API errors.
- Major pages use consistent card, table, toolbar, form, sheet/dialog, spacing, and typography patterns.
- API calls still go through `web/src/lib/api.ts`.
- Setup flow still displays the generated token before navigating into the app.
- Channel, key, chat, log, dashboard, setup, login, and settings pages remain functional.
- The codebase has clearer module boundaries than before the refactor.
- Each child task has its own testable acceptance criteria and build validation.

## Out of Scope
- Backend API redesign.
- Database/store changes.
- Adding new product features beyond what is needed for the refactor.
- Replacing React/Vite or installing a different component library.

## Open Questions
- For the first child task, should the UI foundation aim for strict shadcn defaults or a restrained custom operational-console variant built on shadcn-style primitives?
