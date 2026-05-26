import { DataTable } from "../../components/ui/data-table";
import { Badge } from "../../components/ui/badge";
import type { RequestLog } from "../../lib/types";

export function LogTable({ logs, t }: { logs: RequestLog[]; t: (key: string) => string }) {
  return (
    <DataTable
      empty={t("empty")}
      headers={[t("latestRequest"), t("channelName"), t("model"), t("upstreamModel"), t("status"), t("duration"), t("attempts")]}
      rows={logs.map((log) => [
        formatTime(log.ts),
        log.channelName || "-",
        log.downstreamModel,
        log.upstreamModel || "-",
        <Badge key={`${log.id}-status`} variant={statusVariant(log.statusCode)}>
          {log.statusCode}
        </Badge>,
        `${log.durationMs}ms`,
        log.attempts,
      ])}
    />
  );
}

function statusVariant(statusCode: number) {
  if (statusCode >= 200 && statusCode < 400) {
    return "success";
  }
  if (statusCode >= 400) {
    return "danger";
  }
  return "muted";
}

function formatTime(ts: number) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(ts));
}
