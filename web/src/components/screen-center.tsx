import { ReactNode } from "react";

export function ScreenCenter({ children }: { children: ReactNode }) {
  return <main className="screen-center">{children}</main>;
}
