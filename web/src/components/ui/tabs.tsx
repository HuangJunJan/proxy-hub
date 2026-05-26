import * as TabsPrimitive from "@radix-ui/react-tabs";
import { ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Tabs({
  children,
  onValueChange,
  value,
}: {
  children: ReactNode;
  onValueChange: (value: string) => void;
  value: string;
}) {
  return (
    <TabsPrimitive.Root className="ui-tabs" onValueChange={onValueChange} value={value}>
      {children}
    </TabsPrimitive.Root>
  );
}

export function TabsList({ children }: { children: ReactNode }) {
  return <TabsPrimitive.List className="ui-tabs-list">{children}</TabsPrimitive.List>;
}

export function TabsTrigger({
  children,
  value,
}: {
  children: ReactNode;
  value: string;
}) {
  return (
    <TabsPrimitive.Trigger className={cn("ui-button ui-button-ghost ui-tabs-trigger")} value={value}>
      {children}
    </TabsPrimitive.Trigger>
  );
}

export function TabsContent({ children, value }: { children: ReactNode; value: string }) {
  return (
    <TabsPrimitive.Content className="ui-tabs-content" value={value}>
      {children}
    </TabsPrimitive.Content>
  );
}
