import { useEffect, useState } from "react";
import type { RequestLog } from "./types";

export function useLiveRequests(limit = 500) {
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const source = new EventSource("/api/admin/requests/stream", { withCredentials: true });
    source.addEventListener("open", () => setConnected(true));
    source.addEventListener("error", () => setConnected(false));
    source.addEventListener("request", (event) => {
      const message = event as MessageEvent<string>;
      const entry = JSON.parse(message.data) as RequestLog;
      setLogs((current) => [entry, ...current].slice(0, limit));
    });
    return () => {
      source.close();
      setConnected(false);
    };
  }, [limit]);

  return { connected, logs };
}
