# Implementation Plan: Frontend App Shell Navigation

## Checklist
1. Review `AppShell`, navigation metadata, and responsive styles.
2. Refine AppShell markup only where it improves structure/accessibility.
3. Update shell CSS for sidebar, nav, topbar, controls, and mobile layout.
4. Build with `pnpm --dir web build`.
5. Commit app shell changes.

## Validation
```bash
pnpm --dir web build
```

## Rollback Points
- `web/src/components/layout/app-shell.tsx`
- `web/src/styles.css`
