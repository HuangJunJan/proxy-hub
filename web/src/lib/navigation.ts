import type { LucideIcon } from "lucide-react";
import { BarChart3, KeyRound, MessageSquare, Route, ScrollText, Settings } from "lucide-react";

export interface ConsoleRoute {
  icon: LucideIcon;
  labelKey: string;
  path: string;
}

export const consoleRoutes: ConsoleRoute[] = [
  { icon: BarChart3, labelKey: "dashboard", path: "/dashboard" },
  { icon: Route, labelKey: "channels", path: "/channels" },
  { icon: MessageSquare, labelKey: "chat", path: "/chat" },
  { icon: KeyRound, labelKey: "keys", path: "/keys" },
  { icon: ScrollText, labelKey: "logs", path: "/logs" },
  { icon: Settings, labelKey: "settings", path: "/settings" },
];

export function titleForPath(pathname: string) {
  const route = consoleRoutes.find((item) => item.path === pathname);
  return route?.labelKey ?? "dashboard";
}
