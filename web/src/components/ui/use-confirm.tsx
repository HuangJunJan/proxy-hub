import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "./alert-dialog";

export type ConfirmTone = "default" | "destructive";

export interface ConfirmOptions {
  title: string;
  description: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  tone?: ConfirmTone;
}

export function useConfirm() {
  const [pending, setPending] = useState<ConfirmOptions | null>(null);
  const resolverRef = useRef<((confirmed: boolean) => void) | null>(null);

  const close = useCallback((confirmed: boolean) => {
    const resolve = resolverRef.current;
    resolverRef.current = null;
    setPending(null);
    resolve?.(confirmed);
  }, []);

  const confirm = useCallback((options: ConfirmOptions) => {
    resolverRef.current?.(false);
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve;
      setPending(options);
    });
  }, []);

  useEffect(() => {
    return () => {
      resolverRef.current?.(false);
      resolverRef.current = null;
    };
  }, []);

  const dialog = pending ? (
    <AlertDialog open onOpenChange={(open) => !open && close(false)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{pending.title}</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div>{pending.description}</div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={() => close(false)}>{pending.cancelLabel}</AlertDialogCancel>
          <AlertDialogAction
            className={pending.tone === "destructive" ? "ui-button-destructive" : undefined}
            onClick={() => close(true)}
          >
            {pending.confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  ) : null;

  return { confirm, confirmDialog: dialog };
}
