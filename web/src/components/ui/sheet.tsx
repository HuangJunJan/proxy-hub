import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { ComponentPropsWithoutRef, ElementRef, forwardRef, HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Button } from "./button";

export const Sheet = DialogPrimitive.Root;
export const SheetTrigger = DialogPrimitive.Trigger;
export const SheetClose = DialogPrimitive.Close;

export function SheetPortal({ children }: { children: ReactNode }) {
  return <DialogPrimitive.Portal>{children}</DialogPrimitive.Portal>;
}

export const SheetOverlay = forwardRef<
  ElementRef<typeof DialogPrimitive.Overlay>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay className={cn("ui-sheet-overlay", className)} ref={ref} {...props} />
));

SheetOverlay.displayName = "SheetOverlay";

export const SheetContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ children, className, ...props }, ref) => (
  <SheetPortal>
    <SheetOverlay />
    <DialogPrimitive.Content className={cn("ui-sheet-content", className)} ref={ref} {...props}>
      {children}
      <DialogPrimitive.Close asChild>
        <Button aria-label="Close" className="ui-sheet-close" size="icon" type="button" variant="ghost">
          <X size={16} />
        </Button>
      </DialogPrimitive.Close>
    </DialogPrimitive.Content>
  </SheetPortal>
));

SheetContent.displayName = "SheetContent";

export function SheetHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("ui-sheet-header", className)} {...props} />;
}

export const SheetTitle = forwardRef<
  ElementRef<typeof DialogPrimitive.Title>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title className={cn("ui-card-title", className)} ref={ref} {...props} />
));

SheetTitle.displayName = "SheetTitle";
