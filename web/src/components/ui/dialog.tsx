import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { ReactNode } from "react";
import { Button } from "./button";

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
    <DialogPrimitive.Root
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          onClose?.();
        }
      }}
      open={open}
    >
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="ui-dialog-overlay" />
        <DialogPrimitive.Content className="ui-dialog ui-card">
          <div className="ui-dialog-header">
            <DialogPrimitive.Title className="ui-card-title">{title}</DialogPrimitive.Title>
            {onClose && (
              <DialogPrimitive.Close asChild>
                <Button aria-label="Close" size="icon" type="button" variant="ghost">
                  <X size={16} />
                </Button>
              </DialogPrimitive.Close>
            )}
          </div>
          <div className="ui-dialog-content">{children}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
