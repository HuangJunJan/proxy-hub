import { useEffect, useMemo, useState } from "react";
import { Activity, Gauge, Radio, ShieldCheck } from "lucide-react";
import { Card, CardContent } from "../components/ui/card";
import { DataTable } from "../components/ui/data-table";
import { Toolbar } from "../components/ui/toolbar";
import { MetricCard } from "../features/dashboard/metric-card";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { ChannelSummary, ChannelsResponse } from "../lib/types";

const emptyChannels: ChannelsResponse = { "chatgpt-oauth": [], "openai-api": [] };

export function DashboardPage() {
  const { t } = useAppContext();
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [stats, setStats] = useState<ChannelSummary[]>([]);
  const [error, setError] = useState("");
  const enabledChannels = useMemo(
    () => [...channels["openai-api"], ...channels["chatgpt-oauth"]].filter((channel) => !channel.disabled),
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
  const successRate = totals.requests > 0 ? Math.round((totals.successes / totals.requests) * 100) : 0;
  const errorRate = totals.requests > 0 ? Math.round((totals.failures / totals.requests) * 100) : 0;
  const routeStatus =
    enabledChannels.length === 0 ? t("notConfigured") : errorRate > 0 || totals.failures > 0 ? t("degraded") : t("operational");
  const busiestChannel = stats.reduce<ChannelSummary | undefined>(
    (current, item) => (!current || item.requests > current.requests ? item : current),
    undefined,
  );

  async function refresh() {
    try {
      const [nextStats, nextChannels] = await Promise.all([api.channelStats("24h"), api.channels()]);
      setStats(nextStats);
      setChannels(nextChannels);
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <section className="stack">
      <Toolbar onRefresh={refresh} refreshLabel={t("refresh")} title={t("dashboard")} />
      {error && <p className="error-text">{error}</p>}
      <div className="status-grid">
        <Card className="status-panel status-panel-primary">
          <CardContent>
            <div className="status-panel-heading">
              <span className="status-icon">
                <ShieldCheck size={18} />
              </span>
              <span>{t("healthyRoute")}</span>
            </div>
            <strong>{routeStatus}</strong>
            <div className="progress-track">
              <span style={{ width: `${successRate}%` }} />
            </div>
            <small>{t("successRate")} {successRate}%</small>
          </CardContent>
        </Card>
        <Card className="status-panel">
          <CardContent>
            <div className="status-panel-heading">
              <span className="status-icon">
                <Radio size={18} />
              </span>
              <span>{t("activeChannels")}</span>
            </div>
            <strong>{enabledChannels.length.toLocaleString()}</strong>
            <small>{busiestChannel?.channelName ?? t("empty")}</small>
          </CardContent>
        </Card>
        <Card className="status-panel">
          <CardContent>
            <div className="status-panel-heading">
              <span className="status-icon">
                <Gauge size={18} />
              </span>
              <span>{t("avgLatency")}</span>
            </div>
            <strong>{averageLatency(stats)}ms</strong>
            <small>{t("requests")} {totals.requests.toLocaleString()}</small>
          </CardContent>
        </Card>
        <Card className="status-panel">
          <CardContent>
            <div className="status-panel-heading">
              <span className="status-icon status-icon-danger">
                <Activity size={18} />
              </span>
              <span>{t("errorRate")}</span>
            </div>
            <strong>{errorRate}%</strong>
            <small>{t("failures")} {totals.failures.toLocaleString()}</small>
          </CardContent>
        </Card>
      </div>
      <div className="metric-grid">
        <MetricCard label={t("requests")} value={totals.requests} />
        <MetricCard label={t("successes")} tone="good" value={totals.successes} />
        <MetricCard label={t("failures")} tone="bad" value={totals.failures} />
        <MetricCard label={t("totalTokens")} value={totals.tokens} />
      </div>
      <Card>
        <CardContent>
          <DataTable
            empty={t("empty")}
            headers={[t("channelName"), t("requests"), t("successes"), t("failures"), t("avgLatency"), t("totalTokens")]}
            rows={stats.map((item) => [
              item.channelName,
              item.requests,
              item.successes,
              item.failures,
              `${item.avgDurationMs}ms`,
              item.totalTokens,
            ])}
          />
        </CardContent>
      </Card>
    </section>
  );
}

function averageLatency(stats: ChannelSummary[]) {
  if (stats.length === 0) {
    return 0;
  }
  const totalRequests = stats.reduce((sum, item) => sum + item.requests, 0);
  if (totalRequests === 0) {
    return Math.round(stats.reduce((sum, item) => sum + item.avgDurationMs, 0) / stats.length);
  }
  return Math.round(stats.reduce((sum, item) => sum + item.avgDurationMs * item.requests, 0) / totalRequests);
}
