import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./card";

type SectionCardProps = Omit<HTMLAttributes<HTMLDivElement>, "title"> & {
  contentClassName?: string;
  description?: ReactNode;
  headerClassName?: string;
  title: ReactNode;
};

export function SectionCard({
  children,
  className,
  contentClassName,
  description,
  headerClassName,
  title,
}: SectionCardProps) {
  return (
    <Card className={className}>
      <CardHeader className={headerClassName}>
        <CardTitle>{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent className={contentClassName}>{children}</CardContent>
    </Card>
  );
}
