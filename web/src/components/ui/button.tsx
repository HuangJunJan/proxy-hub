import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { ButtonHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/cn";

export const buttonVariants = cva("ui-button", {
  defaultVariants: {
    size: "default",
    variant: "default",
  },
  variants: {
    size: {
      default: "ui-button-default-size",
      icon: "ui-button-icon",
      sm: "ui-button-sm",
    },
    variant: {
      default: "ui-button-default",
      destructive: "ui-button-destructive",
      ghost: "ui-button-ghost",
      outline: "ui-button-outline",
    },
  },
});

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ asChild = false, className, size, variant, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp className={cn(buttonVariants({ className, size, variant }))} ref={ref} {...props} />;
  },
);

Button.displayName = "Button";
