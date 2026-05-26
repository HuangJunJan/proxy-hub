# Component Guidelines

> Component conventions for the Proxy Hub console.

---

## Overview

The frontend uses shadcn's source-component model: components are local code under `web/src/components/ui`, configured by `web/components.json`, with Radix primitives where accessibility or overlay behavior matters. The app should look like a dense operational console, not a marketing page.

---

## Component Layers

- `components/ui`: reusable UI primitives and small shared widgets.
- `components/layout`: cross-route layout such as `AppShell`.
- `features/<feature>`: domain-specific reusable sections.
- `pages/*-page.tsx`: route orchestration, page-local form state, and API calls.

---

## shadcn UI Pattern

- Keep shadcn-compatible source components in `web/src/components/ui`.
- Use `web/src/lib/cn.ts` for class composition; it must remain `clsx` + `tailwind-merge`.
- Use `class-variance-authority` for variant-heavy primitives such as `Button`.
- Use Radix primitives for dialogs, sheets, tabs, menus, and other interactive accessibility-sensitive controls.
- Keep `web/components.json` in sync with the local aliases:
  - `ui -> src/components/ui`
  - `lib -> src/lib`
  - `utils -> src/lib/cn`

Current examples:

- `button.tsx`: `Slot` + `cva` + `cn`.
- `dialog.tsx`: `@radix-ui/react-dialog` wrapper.
- `sheet.tsx`: Radix Dialog-backed sheet with `Sheet`, `SheetTrigger`, `SheetContent`, `SheetHeader`, and `SheetTitle`.
- `tabs.tsx`: `@radix-ui/react-tabs` wrapper.

---

## Props Conventions

- Shared UI components should accept standard DOM props when useful.
- Use `forwardRef` for primitives that wrap native elements or Radix primitives.
- Use explicit prop objects for small project-specific widgets such as `DataTable` and `Toolbar`.
- Keep backend DTO types out of UI primitive props; those belong in `pages` or `features`.

---

## Styling Patterns

- `styles.css` imports Tailwind v4 through `@import "tailwindcss";`.
- Theme colors are CSS variables on `:root` and `:root[data-theme="dark"]`.
- UI primitives use stable shared classes such as `ui-button`, `ui-card`, `ui-sheet-content`, and `ui-table`.
- Keep dimensions stable for repeated controls and tables; mobile tables should scroll horizontally instead of overlapping content.
- Use lucide icons inside buttons when an icon exists.

---

## Accessibility

- Modal and sheet overlays must use Radix Dialog primitives, not plain div toggles.
- Icon-only buttons need `aria-label`.
- Tabs should use Radix Tabs primitives.
- Forms should use `Field` for visible labels.

---

## Forbidden Patterns

- Do not replace shadcn-style source components with ad hoc page-local controls.
- Do not create fake sheets or dialogs as static cards when the user interaction is an overlay.
- Do not add nested cards unless the inner card is a repeated data item or modal surface.
- Do not introduce a separate component library without updating this spec and `components.json`.

---

## Common Mistakes

- Calling a card-like side panel `Sheet` without Radix overlay behavior.
- Adding page-specific CSS classes for a control that should be a `components/ui` primitive.
- Putting create/edit forms permanently beside data tables when an action-triggered sheet is the better expansion point.
