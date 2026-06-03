import { Trash2 } from "lucide-react";
import { Button } from "../../components/ui/button";
import { CopyButton } from "../../components/ui/copy-button";
import { DataTable } from "../../components/ui/data-table";
import type { DownstreamKey } from "../../lib/types";

export function KeyTable({
  keys,
  onDelete,
  onEdit,
  onToggle,
  t,
}: {
  keys: DownstreamKey[];
  onDelete: (key: DownstreamKey) => void;
  onEdit: (key: DownstreamKey) => void;
  onToggle: (key: DownstreamKey) => void;
  t: (key: string) => string;
}) {
  return (
    <DataTable
      empty={t("empty")}
      headers={[t("keyName"), t("token"), t("notes"), t("status"), ""]}
      rows={keys.map((key) => ({
        cells: [
          key.name || "-",
          key.tokenMask,
          key.notes || "-",
          <span
            className={
              key.disabled ? "status-text danger" : "status-text success"
            }
          >
            {key.disabled ? t("disabled") : t("enabled")}
          </span>,
          <div className="table-actions" key={key.tokenMask}>
            <CopyButton
              copiedLabel={t("copied")}
              label={t("copy")}
              value={key.token || key.tokenMask}
            />
            <Button
              onClick={() => onEdit(key)}
              size="sm"
              type="button"
              variant="outline"
            >
              {t("edit")}
            </Button>
            <Button
              onClick={() => onToggle(key)}
              size="sm"
              type="button"
              variant="outline"
            >
              {key.disabled ? t("enable") : t("disable")}
            </Button>
            <Button
              onClick={() => onDelete(key)}
              size="sm"
              type="button"
              variant="destructive"
            >
              <Trash2 size={14} />
              {t("delete")}
            </Button>
          </div>,
        ],
        key: key.tokenMask,
      }))}
    />
  );
}
