# PRD: Frontend CSS Cleanup

## Goal
Clean up stale and inconsistent CSS after the frontend refactor phases while preserving the shadcn-style operational-console visual direction.

## Requirements
1. Remove or consolidate obvious stale selectors where safe.
2. Normalize remaining hardcoded radii to foundation variables where practical.
3. Keep responsive behavior intact.
4. Avoid broad risky deletions without tests.

## Acceptance Criteria
- `pnpm --dir web build` passes.
- No obvious stale generic page-error styling remains in active use.
- CSS remains token-driven for shared surfaces and controls.
