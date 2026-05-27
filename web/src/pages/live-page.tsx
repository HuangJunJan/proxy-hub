import { useMemo, useState } from "react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { LogTable } from "../features/logs/log-table";
import { useAppContext } from "../lib/app-context";
import { useLiveRequests } from "../lib/use-live-requests";

type LiveFilter = "all" | "error" | "success";

export function LivePage() {
  const { t } = useAppContext();
  const { connected, logs } = useLiveRequests();
  const [filter, setFilter] = useState<LiveFilter>("all");
  const visibleLogs = useMemo(() => {
    if (filter === "error") {
      return logs.filter((log) => log.statusCode >= 400);
    }
    if (filter === "success") {
      return logs.filter((log) => log.statusCode >= 200 && log.statusCode < 400);
    }
    return logs;
  }, [filter, logs]);
  const latest = logs[0];

  return (
    <section className="stack">
      <div className="toolbar toolbar-actions-only">
        <h2 className="toolbar-title">{t("live")}</h2>
        <div className="toolbar-actions">
          <div className="segmented-control" aria-label={t("filters")}>
            <Button onClick={() => setFilter("all")} type="button" variant={filter === "all" ? "default" : "ghost"}>
              {t("all")}
            </Button>
            <Button onClick={() => setFilter("success")} type="button" variant={filter === "success" ? "default" : "ghost"}>
              {t("successOnly")}
            </Button>
            <Button onClick={() => setFilter("error")} type="button" variant={filter === "error" ? "default" : "ghost"}>
              {t("errorsOnly")}
            </Button>
          </div>
          <Badge className={connected ? "live-badge is-live" : "live-badge"} variant={connected ? "success" : "muted"}>
            {connected ? t("streamConnected") : t("streamDisconnected")}
          </Badge>
        </div>
      </div>
      <div className="live-summary">
        <Card className="status-panel">
          <CardContent>
            <div className="status-panel-heading">
              <span>{t("liveBuffer")}</span>
            </div>
            <strong>{logs.length.toLocaleString()}</strong>
            <small>{t("recentEvents")}</small>
          </CardContent>
        </Card>
        <Card className="status-panel status-panel-wide">
          <CardContent>
            <div className="status-panel-heading">
              <span>{t("latestRequest")}</span>
            </div>
            <strong>{latest ? `${latest.statusCode} · ${latest.downstreamModel}` : t("empty")}</strong>
            <small>{latest?.channelName ?? "-"}</small>
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardContent>
          <LogTable logs={visibleLogs} t={t} />
        </CardContent>
      </Card>
    </section>
  );
}
