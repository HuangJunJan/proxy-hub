import { HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "success" | "danger" | "muted";
}

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return <span className={cn("ui-badge", `ui-badge-${variant}`, className)} {...props} />;
}
