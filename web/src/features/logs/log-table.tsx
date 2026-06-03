import { Badge } from "../../components/ui/badge";
import { DataTable } from "../../components/ui/data-table";
import type { RequestLog } from "../../lib/types";

export function LogTable({
  className,
  logs,
  t,
}: {
  className?: string;
  logs: RequestLog[];
  t: (key: string) => string;
}) {
  return (
    <DataTable
      className={className ?? "request-log-table-wrap"}
      empty={t("empty")}
      headers={[
        t("time"),
        t("channelName"),
        t("model"),
        t("status"),
        t("duration"),
        t("token"),
      ]}
      rows={logs.map((log) => ({
        cells: [
          <StackCell
            key={`${log.id}-time`}
            primary={formatDate(log.ts)}
            secondary={formatTime(log.ts)}
          />,
          <StackCell
            key={`${log.id}-channel`}
            primary={log.channelName || "-"}
            secondary={log.channelType || undefined}
          />,
          <StackCell
            key={`${log.id}-model`}
            primary={log.downstreamModel || "-"}
            secondary={modelSecondary(log)}
          />,
          <div key={`${log.id}-status`} className="log-type-cell">
            <Badge variant={statusVariant(log.statusCode)}>
              {log.statusCode}
            </Badge>
            {log.attempts > 1 && (
              <span className="log-attempts">x{log.attempts}</span>
            )}
          </div>,
          <ValueCell
            key={`${log.id}-duration`}
            value={formatMS(log.durationMs)}
          />,
          <TokenCell key={`${log.id}-tokens`} log={log} t={t} />,
        ],
        key: log.id,
      }))}
      tableClassName="request-log-table"
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

function modelSecondary(log: RequestLog) {
  const parts = [
    log.upstreamModel && log.upstreamModel !== log.downstreamModel
      ? log.upstreamModel
      : undefined,
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(" / ") : undefined;
}

function formatMS(value: number | undefined) {
  if (value === undefined || value === null) {
    return "-";
  }
  return `${value.toLocaleString()}ms`;
}

function formatDate(ts: number) {
  return new Intl.DateTimeFormat(undefined, {
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(ts));
}

function formatTime(ts: number) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(ts));
}

function StackCell({
  primary,
  secondary,
}: {
  primary: string;
  secondary?: string;
}) {
  return (
    <span className="table-cell-stack log-stack-cell">
      <strong>{primary}</strong>
      {secondary && <span>{secondary}</span>}
    </span>
  );
}

function ValueCell({ muted, value }: { muted?: boolean; value: string }) {
  return <span className={muted ? "muted-text" : undefined}>{value}</span>;
}

function TokenCell({
  log,
  t,
}: {
  log: RequestLog;
  t: (key: string) => string;
}) {
  const rows = [
    [t("inputToken"), log.promptTokens],
    [t("outputToken"), log.completionTokens],
    [t("reasoningToken"), log.reasoningTokens],
    [t("totalToken"), log.totalTokens],
  ] as const;
  return (
    <span className="log-token-cell">
      {rows.map(([label, value]) => (
        <span className="log-token-row" key={label}>
          <span>{label}</span>
          <strong>{formatToken(value)}</strong>
        </span>
      ))}
    </span>
  );
}

function formatToken(value: number | undefined) {
  return value === undefined || value === null ? "-" : value.toLocaleString();
}
