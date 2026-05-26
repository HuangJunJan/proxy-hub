import { InputHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/cn";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => <input className={cn("ui-input", className)} ref={ref} {...props} />,
);

Input.displayName = "Input";
