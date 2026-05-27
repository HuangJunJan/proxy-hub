import { RefreshCw } from "lucide-react";
import { ReactNode } from "react";
import { Button } from "./button";

export function Toolbar({
  actions,
  onRefresh,
  refreshLabel,
  title,
}: {
  actions?: ReactNode;
  onRefresh: () => void;
  refreshLabel: string;
  title: string;
}) {
  return (
    <div className="toolbar toolbar-actions-only">
      <h2 className="toolbar-title">{title}</h2>
      <div className="toolbar-actions">
        {actions}
        <Button onClick={onRefresh} type="button" variant="outline">
          <RefreshCw size={16} />
          {refreshLabel}
        </Button>
      </div>
    </div>
  );
}
