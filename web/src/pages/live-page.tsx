import { Badge } from "../components/ui/badge";
import { Card, CardContent } from "../components/ui/card";
import { LogTable } from "../features/logs/log-table";
import { useAppContext } from "../lib/app-context";
import { useLiveRequests } from "../lib/use-live-requests";

export function LivePage() {
  const { t } = useAppContext();
  const { connected, logs } = useLiveRequests();

  return (
    <section className="stack">
      <div className="toolbar">
        <h2>{t("live")}</h2>
        <Badge variant={connected ? "success" : "muted"}>{connected ? t("streamConnected") : t("streamDisconnected")}</Badge>
      </div>
      <Card>
        <CardContent>
          <LogTable logs={logs} t={t} />
        </CardContent>
      </Card>
    </section>
  );
}
