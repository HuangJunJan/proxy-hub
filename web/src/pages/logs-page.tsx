import { useEffect, useState } from "react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { Toolbar } from "../components/ui/toolbar";
import { LogTable } from "../features/logs/log-table";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { LogsResponse } from "../lib/types";

const LOG_LIMIT = "100";

interface LogFilters {
  channel: string;
  apiKey: string;
  model: string;
  endpoint: string;
  requestType: string;
  statusClass: string;
  status: string;
  errorKind: string;
  from: string;
  to: string;
}

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
      <Card className="log-filter-card">
        <CardContent>
          <div className="log-filter-heading">
            <div>
              <h3>{t("filters")}</h3>
              <span>{t("logFilterHint")}</span>
            </div>
          </div>
          <div className="log-filter-grid">
            <Field label={t("channelName")}>
              <Input value={filters.channel} onChange={(event) => updateFilter("channel", event.target.value)} />
            </Field>
            <Field label={t("apiKeyShort")}>
              <Input value={filters.apiKey} onChange={(event) => updateFilter("apiKey", event.target.value)} />
            </Field>
            <Field label={t("model")}>
              <Input value={filters.model} onChange={(event) => updateFilter("model", event.target.value)} />
            </Field>
            <Field label={t("endpoint")}>
              <Input value={filters.endpoint} onChange={(event) => updateFilter("endpoint", event.target.value)} />
            </Field>
            <Field label={t("requestType")}>
              <Select value={filters.requestType} onChange={(event) => updateFilter("requestType", event.target.value)}>
                <option value="">{t("all")}</option>
                <option value="chat.completions">chat.completions</option>
                <option value="responses">responses</option>
              </Select>
            </Field>
            <Field label={t("statusGroup")}>
              <Select value={filters.statusClass} onChange={(event) => updateFilter("statusClass", event.target.value)}>
                <option value="">{t("all")}</option>
                <option value="success">{t("successOnly")}</option>
                <option value="error">{t("errorsOnly")}</option>
              </Select>
            </Field>
            <Field label={t("statusCode")}>
              <Input inputMode="numeric" value={filters.status} onChange={(event) => updateFilter("status", event.target.value)} />
            </Field>
            <Field label={t("errorKind")}>
              <Input value={filters.errorKind} onChange={(event) => updateFilter("errorKind", event.target.value)} />
            </Field>
            <Field label={t("fromTime")}>
              <Input type="datetime-local" value={filters.from} onChange={(event) => updateFilter("from", event.target.value)} />
            </Field>
            <Field label={t("toTime")}>
              <Input type="datetime-local" value={filters.to} onChange={(event) => updateFilter("to", event.target.value)} />
            </Field>
          </div>
          <div className="log-filter-actions">
            <Button onClick={() => void refresh()} type="button">
              {t("apply")}
            </Button>
            <Button onClick={clearFilters} type="button" variant="outline">
              {t("clear")}
            </Button>
          </div>
        </CardContent>
      </Card>
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
