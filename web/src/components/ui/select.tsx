import { SelectHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/cn";

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, ...props }, ref) => <select className={cn("ui-select", className)} ref={ref} {...props} />,
);

Select.displayName = "Select";
