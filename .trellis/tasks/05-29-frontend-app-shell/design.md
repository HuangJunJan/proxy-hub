# Design: Frontend App Shell Navigation

## Boundaries
- Modify `web/src/components/layout/app-shell.tsx` and shell-related styles in `web/src/styles.css`.
- Keep route definitions in `web/src/lib/navigation.ts` unchanged unless labels/icons need compatible use.
- Do not move route content or API logic in this task.

## Layout
- Sidebar remains persistent on desktop.
- Topbar continues to show current route title/icon and global controls.
- On narrow screens, sidebar becomes a compact top section and nav wraps without content overlap.

## Styling
- Use UI foundation tokens from the previous child task.
- Keep shell flat and operational: thin borders, neutral surfaces, compact spacing.
- Avoid decorative effects and layout-shifting animations.

## Compatibility
- Preserve `AppShell` export and route usage.
- Preserve all existing user actions.
