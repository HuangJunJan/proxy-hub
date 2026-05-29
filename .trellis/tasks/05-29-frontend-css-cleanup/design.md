# Design: Frontend CSS Cleanup

## Strategy
Perform conservative cleanup only. This codebase has no visual regression tests, so avoid deleting large blocks. Prefer token normalization and removing selectors proven unused by search.

## Boundaries
- Primary file: `web/src/styles.css`.
- No behavior changes.
