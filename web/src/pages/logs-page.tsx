import { useEffect, useState } from "react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Toolbar } from "../components/ui/toolbar";
import { LogTable } from "../features/logs/log-table";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { LogsResponse } from "../lib/types";

export function LogsPage() {
  const { t } = useAppContext();
  const [channel, setChannel] = useState("");
  const [error, setError] = useState("");
  const [logs, setLogs] = useState<LogsResponse>({ items: [], limit: 100, page: 1 });
  const [status, setStatus] = useState("");

  async function refresh() {
    const params = new URLSearchParams();
    if (channel) {
      params.set("channel", channel);
    }
    if (status) {
      params.set("status", status);
    }
    params.set("limit", "100");
    try {
      setLogs(await api.logs(params));
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
      <Toolbar onRefresh={refresh} refreshLabel={t("refresh")} title={t("logs")} />
      <Card>
        <CardContent>
          <div className="filterbar">
            <Input placeholder={t("channelName")} value={channel} onChange={(event) => setChannel(event.target.value)} />
            <Input placeholder={t("status")} value={status} onChange={(event) => setStatus(event.target.value)} />
            <Button onClick={refresh} type="button">
              {t("apply")}
            </Button>
          </div>
        </CardContent>
      </Card>
      {error && <p className="error-text">{error}</p>}
      <Card>
        <CardContent>
          <LogTable logs={logs.items} t={t} />
        </CardContent>
      </Card>
    </section>
  );
}
