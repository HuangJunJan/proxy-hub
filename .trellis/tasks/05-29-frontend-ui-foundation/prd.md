# PRD: Frontend UI Foundation

## Goal
Establish a consistent UI foundation for the admin console using local shadcn-style source components, normalized design tokens, and predictable global layout/style primitives.

## User Value
- Pages should stop feeling visually inconsistent or "weird".
- Future page refactors can reuse a stable visual language instead of adding one-off CSS.
- The app should look like a coherent shadcn-style admin dashboard while retaining a dense operational-console feel.

## Confirmed Facts
- This is the first child task of `.trellis/tasks/05-29-frontend-refactor`.
- The frontend already has local UI primitives in `web/src/components/ui`.
- The project uses Radix primitives for dialog, sheet, and tabs.
- `web/components.json` is configured for shadcn source components.
- Current visual styling is concentrated in `web/src/styles.css` with many custom `.ui-*` classes.
- Existing specs require operational-console style: flat, restrained, compact, thin borders, neutral surfaces, no marketing-style gradients/heavy shadows.
- The user selected visual direction B: primarily shadcn-style primitives with a compact operational-console variant; style may be adjusted based on actual page display quality.

## Requirements
1. Define or normalize global design tokens for background, foreground, card, border, muted, accent, success, danger, focus, and dark mode.
2. Align core primitives visually: Button, Card, Input, Select, Textarea, Badge, Table/DataTable, Dialog, Sheet, Tabs, Toast, Toolbar.
3. Keep the UI foundation primarily shadcn-style: local source components, Radix where appropriate, token-driven styling, consistent variants and states.
4. Allow compact operational-console adjustments where they improve dense data display, readability, and usability.
5. Keep components local under `web/src/components/ui`; do not introduce a separate component library.
6. Preserve existing component public APIs unless a small compatible extension is needed.
7. Keep Radix-backed accessibility behavior for Dialog, Sheet, and Tabs.
8. Keep the global axios error toast behavior intact.
9. Avoid broad page-level behavioral changes in this phase; page-specific restructuring belongs to later child tasks.
10. Improve visual consistency enough that existing pages immediately look more coherent after this phase.

## Acceptance Criteria
- `pnpm --dir web build` passes.
- Core UI primitives share consistent spacing, borders, radius, focus states, hover states, and disabled states.
- Light and dark themes remain usable.
- Global toast is positioned and styled consistently with the rest of the UI.
- Existing pages still render without TypeScript or runtime integration changes.
- No backend API behavior changes.
- The result remains recognizably shadcn-based while being compact enough for admin/log/config screens.

## Out of Scope
- Route page decomposition.
- App shell navigation restructuring.
- API hook extraction.
- Removing all stale CSS; that belongs to the final CSS cleanup child task.
