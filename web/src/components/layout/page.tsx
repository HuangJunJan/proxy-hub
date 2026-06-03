import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Page({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={cn("page", className)} {...props} />;
}

export function PageHeader({
  actions,
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & { actions?: ReactNode }) {
  return (
    <div className={cn("page-header", className)}>
      <div className="page-heading" {...props} />
      {actions ? <div className="page-header-actions">{actions}</div> : null}
    </div>
  );
}

export function PageTitle({
  className,
  visuallyHidden,
  ...props
}: HTMLAttributes<HTMLHeadingElement> & { visuallyHidden?: boolean }) {
  return (
    <h2
      className={cn("page-title", visuallyHidden && "sr-only", className)}
      {...props}
    />
  );
}

export function PageDescription({
  className,
  ...props
}: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("page-description", className)} {...props} />;
}

export function PageBody({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("page-body", className)} {...props} />;
}
