import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Card, CardContent } from "../../components/ui/card";

export function StatusPanel({
  children,
  className,
  icon,
  iconClassName,
  label,
  meta,
  value,
}: {
  children?: ReactNode;
  className?: string;
  icon: ReactNode;
  iconClassName?: string;
  label: ReactNode;
  meta?: ReactNode;
  value: ReactNode;
}) {
  return (
    <Card className={cn("status-panel", className)}>
      <CardContent>
        <div className="status-panel-heading">
          <span className={cn("status-icon", iconClassName)}>{icon}</span>
          <span>{label}</span>
        </div>
        <strong>{value}</strong>
        {children}
        {meta ? <small>{meta}</small> : null}
      </CardContent>
    </Card>
  );
}
