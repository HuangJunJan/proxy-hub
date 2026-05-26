import { TextareaHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/cn";

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => <textarea className={cn("ui-textarea", className)} ref={ref} {...props} />,
);

Textarea.displayName = "Textarea";
