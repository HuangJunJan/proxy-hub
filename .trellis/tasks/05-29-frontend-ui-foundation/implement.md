# Implementation Plan: Frontend UI Foundation

## Ordered Checklist
1. Review current UI primitive CSS in `web/src/styles.css` and local primitive source files.
2. Normalize design tokens and base body/link/form element defaults.
3. Refine core primitives:
   - Card
   - Button
   - Input / Select / Textarea
   - Badge
   - DataTable
   - Dialog / Sheet
   - Tabs
   - Toast
   - Toolbar
4. Keep page-specific behavior unchanged, but make existing pages visually benefit from the foundation changes.
5. Run `pnpm --dir web build`.
6. Review `git diff` for unintended backend/API/page-behavior changes.

## Validation Commands
```bash
pnpm --dir web build
```

## Risky Files / Rollback Points
- `web/src/styles.css` — broad visual impact; keep changes targeted to foundation/global classes where possible.
- `web/src/components/ui/*` — preserve public APIs to avoid page breakage.

## Review Gates
- Build passes.
- No page-local generic API error toast is reintroduced.
- No API calls bypass `web/src/lib/api.ts`.
- Setup token visibility behavior remains unchanged.
