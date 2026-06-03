import { useEffect, useMemo, useRef, useState } from "react";
import {
  Page,
  PageBody,
  PageDescription,
  PageHeader,
  PageTitle,
} from "../components/layout/page";
import { PageActions, RefreshButton } from "../components/ui/page-actions";
import { SectionCard } from "../components/ui/section-card";
import { LogFiltersCard, type LogFilters } from "../features/logs/log-filters";
import { LogTable } from "../features/logs/log-table";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { LogsResponse } from "../lib/types";

const LOG_LIMIT = "100";
const DEFAULT_REFRESH_MS = 5000;
const REFRESH_OPTIONS_MS = [5000, 10000, 30000, 60000] as const;

export function LogsPage() {
  const { t } = useAppContext();
  const [filters, setFilters] = useState<LogFilters>(() =>
    createTodayFilters(),
  );
  const [refreshMs, setRefreshMs] = useState(DEFAULT_REFRESH_MS);
  const [logs, setLogs] = useState<LogsResponse>({
    items: [],
    limit: 100,
    page: 1,
  });
  const filtersRef = useRef(filters);

  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

  const refreshLabel = useMemo(
    () => formatRefreshLabel(refreshMs, t),
    [refreshMs, t],
  );

  async function refresh(nextFilters = filtersRef.current) {
    const params = new URLSearchParams();
    appendTextParam(params, "channel", nextFilters.channel);
    appendTextParam(params, "model", nextFilters.model);
    appendTextParam(params, "status", nextFilters.status);
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
    void refresh(filters);
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void refresh();
    }, refreshMs);
    return () => {
      window.clearInterval(timer);
    };
  }, [refreshMs]);

  function updateFilter(name: keyof LogFilters, value: string) {
    setFilters((current) => ({ ...current, [name]: value }));
  }

  function clearFilters() {
    const next = createTodayFilters();
    setFilters(next);
    void refresh(next);
  }

  return (
    <Page className="logs-page">
      <PageHeader
        actions={
          <PageActions>
            <RefreshButton label={t("refresh")} onClick={refresh} />
          </PageActions>
        }
      >
        <PageTitle visuallyHidden>{t("logs")}</PageTitle>
        <PageDescription>{refreshLabel}</PageDescription>
      </PageHeader>
      <PageBody>
        <LogFiltersCard
          filters={filters}
          onApply={() => void refresh()}
          onChange={updateFilter}
          onClear={clearFilters}
          onRefreshIntervalChange={setRefreshMs}
          refreshIntervalMs={refreshMs}
          refreshIntervalLabel={refreshLabel}
          refreshOptionsMs={REFRESH_OPTIONS_MS}
          t={t}
        />
        <SectionCard
          className="logs-table-card"
          description={`${logs.items.length.toLocaleString()} ${t("requests")}`}
          title={t("logs")}
        >
          <LogTable className="logs-table-fill" logs={logs.items} t={t} />
        </SectionCard>
      </PageBody>
    </Page>
  );
}

function createTodayFilters(): LogFilters {
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  const end = new Date();
  return {
    channel: "",
    model: "",
    status: "",
    from: toDateTimeLocal(start),
    to: toDateTimeLocal(end),
  };
}

function toDateTimeLocal(value: Date) {
  const offset = value.getTimezoneOffset();
  const local = new Date(value.getTime() - offset * 60_000);
  return local.toISOString().slice(0, 16);
}

function formatRefreshLabel(refreshMs: number, t: (key: string) => string) {
  return `${t("autoRefresh")}: ${Math.round(refreshMs / 1000)}s`;
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
