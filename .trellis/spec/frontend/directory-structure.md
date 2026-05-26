# Directory Structure

> Frontend module boundaries for `web/src`.

---

## Overview

The console is organized by layer. New pages should not be added as more state and markup inside `App.tsx`; `App.tsx` owns bootstrapping, setup/login/app mode routing, and context wiring only.

---

## Directory Layout

```text
web/
├── components.json              # shadcn configuration
└── src/
    ├── main.tsx                 # React root and BrowserRouter
    ├── App.tsx                  # boot flow, route table, app context provider
    ├── styles.css               # Tailwind v4 import, CSS variables, shared classes
    ├── pages/                   # route-level screens
    │   ├── setup-page.tsx
    │   ├── login-page.tsx
    │   ├── dashboard-page.tsx
    │   ├── channels-page.tsx
    │   ├── keys-page.tsx
    │   ├── logs-page.tsx
    │   ├── live-page.tsx
    │   └── settings-page.tsx
    ├── features/                # reusable domain sections below route level
    │   ├── channels/
    │   ├── dashboard/
    │   └── logs/
    ├── components/
    │   ├── layout/              # app shell and route layout primitives
    │   └── ui/                  # shadcn-style source UI components
    └── lib/                     # API client, DTO types, context, hooks, theme, i18n
```

---

## Module Organization

- Route entries live in `web/src/pages/*-page.tsx`.
- Reusable domain UI that belongs to one product area lives in `web/src/features/<feature>/`.
- Cross-page shell components live in `web/src/components/layout/`.
- shadcn-style primitives and small shared widgets live in `web/src/components/ui/`.
- Shared non-visual code lives in `web/src/lib/`.
- API calls go through `web/src/lib/api.ts`.
- Shared backend DTOs and persisted UI unions live in `web/src/lib/types.ts`.
- New routes are registered in `App.tsx`, but their implementation belongs in `pages/` and optionally `features/`.

---

## Naming Conventions

- Files use kebab-case: `channels-page.tsx`, `metric-card.tsx`, `use-live-requests.ts`.
- Components use PascalCase exports: `ChannelsPage`, `MetricCard`, `DataTable`.
- Route-level files end with `-page.tsx`.
- Custom hooks start with `use`.
- UI primitive files match their component name in lower kebab-case: `button.tsx`, `data-table.tsx`.

---

## Forbidden Patterns

- Do not put page-specific tables, forms, or request state directly in `App.tsx`.
- Do not add new root-level files under `web/src` except boot/global files.
- Do not bypass `lib/api.ts` with page-local `fetch` or `axios` calls.
- Do not duplicate backend DTO shapes in individual pages; add them to `lib/types.ts`.

---

## Examples

- `web/src/pages/channels-page.tsx` owns the `/channels` route and composes `features/channels/channel-list.tsx`.
- `web/src/pages/logs-page.tsx` owns filters and calls `features/logs/log-table.tsx` for the repeated table display.
- `web/src/components/ui/sheet.tsx` wraps Radix Dialog primitives in the shadcn source-component style.
