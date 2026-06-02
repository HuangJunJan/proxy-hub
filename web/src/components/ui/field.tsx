import { ReactNode, useId } from "react";

export function Field({ children, label }: { children: ReactNode; label: string }) {
  const labelId = useId();

  return (
    <div aria-labelledby={labelId} className="ui-field" role="group">
      <span id={labelId}>{label}</span>
      {children}
    </div>
  );
}
