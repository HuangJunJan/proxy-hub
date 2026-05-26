import type { ThemeMode } from "./types";

export function applyTheme(theme: ThemeMode) {
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isDark = theme === "dark" || (theme === "system" && prefersDark);
  document.documentElement.dataset.theme = isDark ? "dark" : "light";
}
