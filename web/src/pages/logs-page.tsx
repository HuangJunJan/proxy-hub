import { useEffect, useState } from "react";
import { Card, CardContent } from "../components/ui/card";
import { Toolbar } from "../components/ui/toolbar";
import { LogFiltersCard, type LogFilters } from "../features/logs/log-filters";
import { LogTable } from "../features/logs/log-table";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { LogsResponse } from "../lib/types";

const LOG_LIMIT = "100";

const emptyFilters: LogFilters = {
  channel: "",
  apiKey: "",
  model: "",
  endpoint: "",
  requestType: "",
  statusClass: "",
  status: "",
  errorKind: "",
  from: "",
  to: "",
};

export function LogsPage() {
  const { t } = useAppContext();
  const [filters, setFilters] = useState<LogFilters>(emptyFilters);
  const [logs, setLogs] = useState<LogsResponse>({ items: [], limit: 100, page: 1 });

  async function refresh(nextFilters = filters) {
    const params = new URLSearchParams();
    appendTextParam(params, "channel", nextFilters.channel);
    appendTextParam(params, "apiKey", nextFilters.apiKey);
    appendTextParam(params, "model", nextFilters.model);
    appendTextParam(params, "endpoint", nextFilters.endpoint);
    appendTextParam(params, "requestType", nextFilters.requestType);
    appendTextParam(params, "statusClass", nextFilters.statusClass);
    appendTextParam(params, "status", nextFilters.status);
    appendTextParam(params, "errorKind", nextFilters.errorKind);
    appendTimeParam(params, "from", nextFilters.from);
    appendTimeParam(params, "to", nextFilters.to);
    params.set("limit", LOG_LIMIT);
    try {
      setLogs(await api.logs(params));
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  function updateFilter(name: keyof LogFilters, value: string) {
    setFilters((current) => ({ ...current, [name]: value }));
  }

  function clearFilters() {
    setFilters(emptyFilters);
    void refresh(emptyFilters);
  }

  return (
    <section className="stack">
      <Toolbar onRefresh={() => void refresh()} refreshLabel={t("refresh")} title={t("logs")} />
      <LogFiltersCard filters={filters} onApply={() => void refresh()} onChange={updateFilter} onClear={clearFilters} t={t} />
      <Card>
        <CardContent>
          <LogTable logs={logs.items} t={t} />
        </CardContent>
      </Card>
    </section>
  );
}

function appendTextParam(params: URLSearchParams, key: string, value: string) {
  const trimmed = value.trim();
  if (trimmed) {
    params.set(key, trimmed);
  }
}

function appendTimeParam(params: URLSearchParams, key: string, value: string) {
  if (!value) {
    return;
  }
  const ms = new Date(value).getTime();
  if (!Number.isNaN(ms)) {
    params.set(key, String(ms));
  }
}
