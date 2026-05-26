import { DataTable } from "../../components/ui/data-table";
import type { RequestLog } from "../../lib/types";

export function LogTable({ logs, t }: { logs: RequestLog[]; t: (key: string) => string }) {
  return (
    <DataTable
      empty={t("empty")}
      headers={[t("channelName"), t("model"), t("upstreamModel"), t("status"), t("duration"), t("attempts")]}
      rows={logs.map((log) => [
        log.channelName || "-",
        log.downstreamModel,
        log.upstreamModel || "-",
        log.statusCode,
        `${log.durationMs}ms`,
        log.attempts,
      ])}
    />
  );
}
