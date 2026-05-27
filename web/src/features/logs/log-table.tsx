import { Badge } from "../../components/ui/badge";
import { DataTable } from "../../components/ui/data-table";
import type { RequestLog } from "../../lib/types";

export function LogTable({ logs, t }: { logs: RequestLog[]; t: (key: string) => string }) {
  return (
    <DataTable
      className="request-log-table-wrap"
      empty={t("empty")}
      headers={[
        t("apiKeyShort"),
        t("model"),
        t("reasoningEffort"),
        t("endpoint"),
        t("requestType"),
        t("token"),
        t("firstToken"),
        t("duration"),
        t("time"),
        t("userAgent"),
      ]}
      rows={logs.map((log) => [
        <StackCell key={`${log.id}-key`} primary={log.apiKeyName || log.apiKeyTokenMask || "-"} secondary={log.apiKeyName ? log.apiKeyTokenMask : undefined} />,
        <StackCell
          key={`${log.id}-model`}
          primary={log.downstreamModel || "-"}
          secondary={modelSecondary(log)}
        />,
        <ValueCell key={`${log.id}-reasoning`} value={log.reasoningEffort || "-"} />,
        <code key={`${log.id}-endpoint`} className="log-endpoint" title={log.endpoint || "-"}>
          {log.endpoint || "-"}
        </code>,
        <div key={`${log.id}-type`} className="log-type-cell">
          <span>{formatRequestType(log)}</span>
          <Badge variant={statusVariant(log.statusCode)}>{log.statusCode}</Badge>
          {log.attempts > 1 && <span className="log-attempts">x{log.attempts}</span>}
        </div>,
        <TokenCell key={`${log.id}-tokens`} log={log} t={t} />,
        <ValueCell key={`${log.id}-first-token`} value={formatMS(log.firstTokenMs)} />,
        <ValueCell key={`${log.id}-duration`} value={formatMS(log.durationMs)} />,
        <StackCell key={`${log.id}-time`} primary={formatDate(log.ts)} secondary={formatTime(log.ts)} />,
        <code key={`${log.id}-ua`} className="log-user-agent" title={log.userAgent || "-"}>
          {log.userAgent || "-"}
        </code>,
      ])}
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
  const parts = [log.channelName, log.upstreamModel && log.upstreamModel !== log.downstreamModel ? log.upstreamModel : undefined].filter(Boolean);
  return parts.length > 0 ? parts.join(" / ") : undefined;
}

function formatRequestType(log: RequestLog) {
  if (log.requestType) {
    return log.requestType;
  }
  return log.isStream ? "stream" : "sync";
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

function StackCell({ primary, secondary }: { primary: string; secondary?: string }) {
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

function TokenCell({ log, t }: { log: RequestLog; t: (key: string) => string }) {
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
