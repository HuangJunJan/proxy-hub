# Design: Frontend API Hooks State Cleanup

## Strategy
Introduce small hooks in `web/src/lib` for repeated request patterns. Keep page ownership of domain state unless a shared hook has a clear contract.

## Initial Hook
- `useAsyncAction`: wraps async actions with a `loading` state and preserves global error handling by rethrowing/catching at call site as needed.

## Boundaries
- Do not introduce a full server-state library in this refactor.
- Do not bypass `lib/api.ts`.
