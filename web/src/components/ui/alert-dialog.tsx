import * as AlertDialogPrimitive from "@radix-ui/react-alert-dialog";
import { ComponentPropsWithoutRef, ElementRef, HTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/cn";
import { buttonVariants } from "./button";

export const AlertDialog = AlertDialogPrimitive.Root;
export const AlertDialogTrigger = AlertDialogPrimitive.Trigger;
export const AlertDialogPortal = AlertDialogPrimitive.Portal;

export const AlertDialogOverlay = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Overlay>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <AlertDialogPrimitive.Overlay className={cn("ui-dialog-overlay", className)} ref={ref} {...props} />
));
AlertDialogOverlay.displayName = AlertDialogPrimitive.Overlay.displayName;

export const AlertDialogContent = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Content>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Content>
>(({ className, ...props }, ref) => (
  <AlertDialogPortal>
    <AlertDialogOverlay />
    <AlertDialogPrimitive.Content className={cn("ui-dialog ui-card", className)} ref={ref} {...props} />
  </AlertDialogPortal>
));
AlertDialogContent.displayName = AlertDialogPrimitive.Content.displayName;

export const AlertDialogHeader = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("ui-dialog-header", className)} {...props} />
);
AlertDialogHeader.displayName = "AlertDialogHeader";

export const AlertDialogFooter = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("confirm-dialog-actions", className)} {...props} />
);
AlertDialogFooter.displayName = "AlertDialogFooter";

export const AlertDialogTitle = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Title>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <AlertDialogPrimitive.Title className={cn("ui-card-title", className)} ref={ref} {...props} />
));
AlertDialogTitle.displayName = AlertDialogPrimitive.Title.displayName;

export const AlertDialogDescription = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Description>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <AlertDialogPrimitive.Description className={cn("confirm-dialog-description", className)} ref={ref} {...props} />
));
AlertDialogDescription.displayName = AlertDialogPrimitive.Description.displayName;

export const AlertDialogAction = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Action>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Action>
>(({ className, ...props }, ref) => (
  <AlertDialogPrimitive.Action className={cn(buttonVariants(), className)} ref={ref} {...props} />
));
AlertDialogAction.displayName = AlertDialogPrimitive.Action.displayName;

export const AlertDialogCancel = forwardRef<
  ElementRef<typeof AlertDialogPrimitive.Cancel>,
  ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Cancel>
>(({ className, ...props }, ref) => (
  <AlertDialogPrimitive.Cancel className={cn(buttonVariants({ variant: "outline" }), className)} ref={ref} {...props} />
));
AlertDialogCancel.displayName = AlertDialogPrimitive.Cancel.displayName;
