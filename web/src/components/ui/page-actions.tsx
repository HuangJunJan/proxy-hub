import { RefreshCw } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Button } from "./button";

export function PageActions({ children, className }: { children?: ReactNode; className?: string }) {
  return <div className={cn("page-actions", className)}>{children}</div>;
}

export function RefreshButton({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void | Promise<void>;
}) {
  return (
    <Button onClick={() => void onClick()} type="button" variant="outline">
      <RefreshCw size={16} />
      {label}
    </Button>
  );
}
