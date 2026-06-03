import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Card, CardContent } from "./card";

export function StatTile({
  className,
  icon,
  iconTone,
  label,
  tone,
  value,
}: {
  className?: string;
  icon?: ReactNode;
  iconTone?: "success";
  label: ReactNode;
  tone?: "danger" | "success";
  value: ReactNode;
}) {
  return (
    <Card className={cn("stat-tile", className)}>
      <CardContent className="stat-tile-content">
        {icon ? (
          <span
            className={cn(
              "stat-tile-icon",
              iconTone === "success" && "is-success",
            )}
          >
            {icon}
          </span>
        ) : null}
        <div className="stat-tile-copy">
          <span className="stat-tile-label">{label}</span>
          <strong
            className={cn(
              "stat-tile-value",
              tone === "success" && "is-success",
              tone === "danger" && "is-danger",
            )}
          >
            {value}
          </strong>
        </div>
      </CardContent>
    </Card>
  );
}
