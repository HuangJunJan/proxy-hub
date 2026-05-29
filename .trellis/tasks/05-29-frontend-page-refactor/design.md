# Design: Frontend Page Refactor

## Boundaries
- Prefer extracting visual/domain sections only; API hook extraction belongs to the next child task.
- Feature components live in `web/src/features/<domain>`.
- Route pages keep local state and submit handlers for this phase unless moving a pure rendering section is straightforward.

## Target Extractions
- Logs filter form into `features/logs/log-filters.tsx`.
- Keys table into `features/keys/key-table.tsx`.
- Chat controls/thread into `features/chat/*` where practical.
- Dashboard summary/status panels into `features/dashboard/*` where practical.

## Compatibility
- Preserve props and DTOs with exact backend field names.
