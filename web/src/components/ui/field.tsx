import { ReactNode } from "react";

export function Field({ children, label }: { children: ReactNode; label: string }) {
  return (
    <label className="ui-field">
      <span>{label}</span>
      {children}
    </label>
  );
}
