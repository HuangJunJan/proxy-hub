import { useEffect, useMemo, useState } from "react";
import { Card, CardContent } from "../components/ui/card";
import { DataTable } from "../components/ui/data-table";
import { Toolbar } from "../components/ui/toolbar";
import { MetricCard } from "../features/dashboard/metric-card";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { ChannelSummary } from "../lib/types";

export function DashboardPage() {
  const { t } = useAppContext();
  const [stats, setStats] = useState<ChannelSummary[]>([]);
  const [error, setError] = useState("");
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

  async function refresh() {
    try {
      setStats(await api.channelStats("24h"));
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
