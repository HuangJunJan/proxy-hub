import type { ReactNode } from "react";
import { ScreenCenter } from "../screen-center";
import { cn } from "../../lib/cn";
import { SectionCard } from "../ui/section-card";

export function AuthScreen({
  children,
  className,
  description,
  title,
}: {
  children: ReactNode;
  className?: string;
  description?: ReactNode;
  title: ReactNode;
}) {
  return (
    <ScreenCenter>
      <SectionCard
        className={cn("auth-panel", className)}
        description={description}
        title={title}
      >
        {children}
      </SectionCard>
    </ScreenCenter>
  );
}
