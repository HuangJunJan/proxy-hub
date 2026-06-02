import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { ComponentPropsWithoutRef, ElementRef, HTMLAttributes, ReactNode, forwardRef } from "react";
import { cn } from "../../lib/cn";
import { Button } from "./button";

export const DialogRoot = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogPortal = DialogPrimitive.Portal;
export const DialogClose = DialogPrimitive.Close;

export const DialogOverlay = forwardRef<
  ElementRef<typeof DialogPrimitive.Overlay>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay className={cn("ui-dialog-overlay", className)} ref={ref} {...props} />
));
DialogOverlay.displayName = DialogPrimitive.Overlay.displayName;

export const DialogContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { showClose?: boolean }
>(({ children, className, showClose = true, ...props }, ref) => (
  <DialogPortal>
    <DialogOverlay />
    <DialogPrimitive.Content className={cn("ui-dialog ui-card", className)} ref={ref} {...props}>
      {children}
      {showClose && (
        <DialogPrimitive.Close asChild>
          <Button aria-label="Close" className="ui-dialog-close" size="icon" type="button" variant="ghost">
            <X size={16} />
          </Button>
        </DialogPrimitive.Close>
      )}
    </DialogPrimitive.Content>
  </DialogPortal>
));
DialogContent.displayName = DialogPrimitive.Content.displayName;

export function DialogHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("ui-dialog-header", className)} {...props} />;
}

export function DialogFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("ui-dialog-footer", className)} {...props} />;
}

export const DialogTitle = forwardRef<
  ElementRef<typeof DialogPrimitive.Title>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title className={cn("ui-card-title", className)} ref={ref} {...props} />
));
DialogTitle.displayName = DialogPrimitive.Title.displayName;

export const DialogDescription = forwardRef<
  ElementRef<typeof DialogPrimitive.Description>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Description className={cn("ui-dialog-description", className)} ref={ref} {...props} />
));
DialogDescription.displayName = DialogPrimitive.Description.displayName;

/**
 * Backward-compatible convenience wrapper for existing pages.
 * New code can use DialogRoot/DialogContent/DialogHeader directly.
 */
export function Dialog({
  children,
  onClose,
  open,
  title,
}: {
  children: ReactNode;
  onClose?: () => void;
  open: boolean;
  title: string;
}) {
  return (
    <DialogRoot
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          onClose?.();
        }
      }}
      open={open}
    >
      <DialogContent showClose={Boolean(onClose)}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="ui-dialog-content">{children}</div>
      </DialogContent>
    </DialogRoot>
  );
}
