# PRD: Frontend App Shell Navigation

## Goal
Refactor the admin console app shell and navigation so the layout feels like a coherent shadcn-style operational dashboard while preserving current routing, theme, language, and logout behavior.

## User Value
- Navigation and page framing should feel stable and professional.
- Users should clearly understand the current section and have predictable access to language/theme/logout controls.
- Responsive behavior should be usable on narrower screens.

## Confirmed Facts
- Parent task: `.trellis/tasks/05-29-frontend-refactor`.
- This follows the UI foundation child task.
- Current shell is `web/src/components/layout/app-shell.tsx`.
- Route metadata lives in `web/src/lib/navigation.ts`.
- `App.tsx` owns boot flow and route table and should not receive page-specific logic.
- Theme and language are stored through `usePersistentState` and app context.

## Requirements
1. Keep existing routes and labels intact.
2. Keep language selector, theme selector, gateway status, and logout available.
3. Improve sidebar/topbar structure and responsive behavior.
4. Keep AppShell as the cross-route layout primitive under `components/layout`.
5. Avoid page-specific changes except where shell spacing requires compatible class updates.
6. Preserve `api.logout()` and `setLoggedOut()` behavior.
7. Preserve current route title/icon behavior.

## Acceptance Criteria
- `pnpm --dir web build` passes.
- AppShell remains the only layout wrapper for authenticated routes.
- Navigation active state remains correct.
- Theme/language controls still update state.
- Logout still returns the user to `/login`.
- Mobile/narrow layout does not overlap controls or content.
