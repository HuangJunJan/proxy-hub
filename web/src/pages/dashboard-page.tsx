import { useEffect, useMemo, useState } from "react";
import { Activity, Gauge, Radio, ShieldCheck } from "lucide-react";
import {
  Page,
  PageBody,
  PageDescription,
  PageHeader,
  PageTitle,
} from "../components/layout/page";
import { DataTable } from "../components/ui/data-table";
import { PageActions, RefreshButton } from "../components/ui/page-actions";
import { SectionCard } from "../components/ui/section-card";
import { MetricCard } from "../features/dashboard/metric-card";
import { StatusPanel } from "../features/dashboard/status-panel";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { ChannelSummary, ChannelsResponse } from "../lib/types";

const emptyChannels: ChannelsResponse = {
  "chatgpt-oauth": [],
  "openai-api": [],
};

export function DashboardPage() {
  const { t } = useAppContext();
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [stats, setStats] = useState<ChannelSummary[]>([]);
  const enabledChannels = useMemo(
    () =>
      [...channels["openai-api"], ...channels["chatgpt-oauth"]].filter(
        (channel) => !channel.disabled,
      ),
    [channels],
  );
  const totals = useMemo(
    () =>
      stats.reduce(
        (acc, item) => ({
          failures: acc.failures + item.failures,
          requests: acc.requests + item.requests,
          successes: acc.successes + item.successes,
          tokens: acc.tokens + item.totalTokens,
        }),
        { failures: 0, requests: 0, successes: 0, tokens: 0 },
      ),
    [stats],
  );
  const successRate =
    totals.requests > 0
      ? Math.round((totals.successes / totals.requests) * 100)
      : 0;
  const errorRate =
    totals.requests > 0
      ? Math.round((totals.failures / totals.requests) * 100)
      : 0;
  const routeStatus =
    enabledChannels.length === 0
      ? t("notConfigured")
      : errorRate > 0 || totals.failures > 0
        ? t("degraded")
        : t("operational");
  const busiestChannel = stats.reduce<ChannelSummary | undefined>(
    (current, item) =>
      !current || item.requests > current.requests ? item : current,
    undefined,
  );

  async function refresh() {
    try {
      const [nextStats, nextChannels] = await Promise.all([
        api.channelStats("24h"),
        api.channels(),
      ]);
      setStats(nextStats);
      setChannels(nextChannels);
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <Page>
      <PageHeader
        actions={
          <PageActions>
            <RefreshButton label={t("refresh")} onClick={refresh} />
          </PageActions>
        }
      >
        <PageTitle visuallyHidden>{t("dashboard")}</PageTitle>
        <PageDescription>{t("gatewayReady")}</PageDescription>
      </PageHeader>
      <PageBody>
        <div className="status-grid">
          <StatusPanel
            className="status-panel-primary"
            icon={<ShieldCheck size={18} />}
            label={t("healthyRoute")}
            meta={`${t("successRate")} ${successRate}%`}
            value={routeStatus}
          >
            <div className="progress-track">
              <span style={{ width: `${successRate}%` }} />
            </div>
          </StatusPanel>
          <StatusPanel
            icon={<Radio size={18} />}
            label={t("activeChannels")}
            meta={busiestChannel?.channelName ?? t("empty")}
            value={enabledChannels.length.toLocaleString()}
          />
          <StatusPanel
            icon={<Gauge size={18} />}
            label={t("avgLatency")}
            meta={`${t("requests")} ${totals.requests.toLocaleString()}`}
            value={`${averageLatency(stats)}ms`}
          />
          <StatusPanel
            icon={<Activity size={18} />}
            iconClassName="status-icon-danger"
            label={t("errorRate")}
            meta={`${t("failures")} ${totals.failures.toLocaleString()}`}
            value={`${errorRate}%`}
          />
        </div>
        <div className="metric-grid">
          <MetricCard label={t("requests")} value={totals.requests} />
          <MetricCard
            label={t("successes")}
            tone="good"
            value={totals.successes}
          />
          <MetricCard
            label={t("failures")}
            tone="bad"
            value={totals.failures}
          />
          <MetricCard label={t("totalTokens")} value={totals.tokens} />
        </div>
        <SectionCard
          description={`${stats.length.toLocaleString()} ${t("channels")}`}
          title={t("channels")}
        >
          <DataTable
            empty={t("empty")}
            headers={[
              t("channelName"),
              t("requests"),
              t("successes"),
              t("failures"),
              t("avgLatency"),
              t("totalTokens"),
            ]}
            rows={stats.map((item) => ({
              cells: [
                item.channelName,
                item.requests,
                item.successes,
                item.failures,
                `${item.avgDurationMs}ms`,
                item.totalTokens,
              ],
              key: item.channelName,
            }))}
          />
        </SectionCard>
      </PageBody>
    </Page>
  );
}

function averageLatency(stats: ChannelSummary[]) {
  if (stats.length === 0) {
    return 0;
  }
  const totalRequests = stats.reduce((sum, item) => sum + item.requests, 0);
  if (totalRequests === 0) {
    return Math.round(
      stats.reduce((sum, item) => sum + item.avgDurationMs, 0) / stats.length,
    );
  }
  return Math.round(
    stats.reduce((sum, item) => sum + item.avgDurationMs * item.requests, 0) /
      totalRequests,
  );
}
