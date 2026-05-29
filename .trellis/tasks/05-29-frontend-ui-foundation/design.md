# Design: Frontend UI Foundation

## Architecture and Boundaries
- Keep the existing React/Vite/TypeScript stack.
- Keep local shadcn-style source components in `web/src/components/ui` as the UI foundation.
- Keep visual tokens and low-level utility classes in `web/src/styles.css` for this phase.
- Avoid page-specific restructuring in this child task; page decomposition is handled by later child tasks.

## Visual Direction
- Primary style: shadcn-style primitives with a compact operational-console variant.
- Use token-driven styling with neutral surfaces, thin borders, restrained shadows, compact spacing, and clear focus states.
- Avoid decorative gradients, heavy shadowing, large marketing cards, and oversized spacing.

## Component Contracts
- Preserve current exported component names and most props:
  - `Button`, `Card`, `Input`, `Select`, `Textarea`, `Badge`, `DataTable`, `Dialog`, `Sheet`, `Tabs`, `Toast`, `Toolbar`.
- Compatible extensions are allowed if they do not require broad page rewrites.
- Radix-backed components keep their accessibility and overlay behavior.

## Styling Strategy
- Normalize core tokens in `:root` and `:root[data-theme="dark"]`.
- Update `.ui-*` classes so primitives share consistent:
  - radius
  - border color
  - background layers
  - hover states
  - focus ring
  - disabled state
  - typography scale
  - table row density
- Do not remove all legacy/page-specific CSS in this task; only adjust foundation classes and obvious global inconsistencies.

## Data Flow and Error Handling
- No API contract changes.
- Global axios error toast remains mounted through `GlobalToast`.
- Page-local generic error toasts must not be reintroduced.

## Compatibility and Rollback
- Since this task is primarily CSS/UI primitive work, rollback can revert `web/src/styles.css` and any small primitive changes.
- Validate with `pnpm --dir web build`.
