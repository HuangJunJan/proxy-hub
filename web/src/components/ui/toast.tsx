import { ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Toast({ children, variant = "default" }: { children: ReactNode; variant?: "default" | "destructive" }) {
  return <div className={cn("ui-toast", variant === "destructive" && "ui-toast-destructive")}>{children}</div>;
}
