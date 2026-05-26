import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { DataTable } from "../../components/ui/data-table";
import type { ChannelHealthResult, ModelEntry, OAuthChannel, OpenAIChannel } from "../../lib/types";

type ChannelListItem = OpenAIChannel | OAuthChannel;

export function ChannelList({
  health,
  items,
  onCheckHealth,
  onDelete,
  onEdit,
  onToggle,
  title,
  type,
  t,
}: {
  health?: Record<string, ChannelHealthResult | undefined>;
  items: ChannelListItem[] | null | undefined;
  onCheckHealth?: (name: string) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
  onEdit?: (item: OpenAIChannel) => void;
  onToggle: (item: ChannelListItem) => Promise<void>;
  title: string;
  type: "chatgpt-oauth" | "openai-api";
  t: (key: string) => string;
}) {
  const safeItems = items ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable
          empty={t("empty")}
          headers={[
            t("channelName"),
            t("status"),
            t("upstream"),
            t("credentials"),
            t("routing"),
            t("visibleModels"),
            t("health"),
            "",
          ]}
          rows={safeItems.map((item) => [
            <div className="table-cell-stack" key={`${item.name}-name`}>
              <strong>{item.name}</strong>
              {item.notes && <span>{item.notes}</span>}
            </div>,
            <StatusText disabled={item.disabled} t={t} />,
            <UpstreamCell item={item} t={t} />,
            credentialLabel(item, t),
            <RoutingCell item={item} t={t} />,
            <ModelList key={`${item.name}-models`} models={item.models} t={t} />,
            <HealthCell health={health?.[item.name]} name={item.name} onCheckHealth={onCheckHealth} t={t} type={type} />,
            <div className="table-actions" key={`${item.name}-actions`}>
              {"base-url" in item && onEdit && (
                <Button onClick={() => onEdit(item)} size="sm" type="button" variant="outline">
                  {t("edit")}
                </Button>
              )}
              <Button onClick={() => void onToggle(item)} size="sm" type="button" variant="outline">
                {item.disabled ? t("enable") : t("disable")}
              </Button>
              <Button onClick={() => void onDelete(item.name)} size="sm" type="button" variant="destructive">
                {t("delete")}
              </Button>
            </div>,
          ])}
        />
      </CardContent>
    </Card>
  );
}

function StatusText({ disabled, t }: { disabled?: boolean; t: (key: string) => string }) {
  return <span className={disabled ? "status-text danger" : "status-text success"}>{disabled ? t("disabled") : t("enabled")}</span>;
}

function UpstreamCell({ item, t }: { item: ChannelListItem; t: (key: string) => string }) {
  if ("base-url" in item) {
    return (
      <code className="table-code" title={item["base-url"]}>
        {item["base-url"]}
      </code>
    );
  }
  return <span className="muted-text">{t("oauthToken")}</span>;
}

function RoutingCell({ item, t }: { item: ChannelListItem; t: (key: string) => string }) {
  return (
    <div className="table-cell-stack">
      {"priority" in item && <span>{`${t("priority")}: ${item.priority || 100}`}</span>}
      <span>{`${t("timeoutSec")}: ${item["timeout-sec"] || 120}`}</span>
    </div>
  );
}

function HealthCell({
  health,
  name,
  onCheckHealth,
  t,
  type,
}: {
  health?: ChannelHealthResult;
  name: string;
  onCheckHealth?: (name: string) => Promise<void>;
  t: (key: string) => string;
  type: "chatgpt-oauth" | "openai-api";
}) {
  if (type !== "openai-api") {
    return <span className="muted-text">{t("manualOnly")}</span>;
  }
  return (
    <div className="table-cell-stack">
      {health && (
        <span className={health.ok ? "status-text success" : "status-text danger"}>
          {health.ok ? `${t("healthy")} ${health.latencyMs ?? 0}ms` : health.error || t("unhealthy")}
        </span>
      )}
      <Button onClick={() => void onCheckHealth?.(name)} size="sm" type="button" variant="outline">
        {t("healthCheck")}
      </Button>
    </div>
  );
}

function credentialLabel(item: ChannelListItem, t: (key: string) => string) {
  if ("api-key-entries" in item) {
    return `${item["api-key-entries"]?.length ?? 0} ${t("apiKeyEntries")}`;
  }
  return item.oauth?.["refresh-token"] || item.oauth?.["access-token"] ? t("configured") : t("notConfigured");
}

function ModelList({ models, t }: { models: ModelEntry[] | null | undefined; t: (key: string) => string }) {
  const safeModels = models ?? [];
  const visible = safeModels.slice(0, 8);
  const hiddenCount = Math.max(0, safeModels.length - visible.length);

  if (safeModels.length === 0) {
    return <span className="muted-text">{t("empty")}</span>;
  }

  return (
    <div className="model-chip-list">
      {visible.map((model) => (
        <span className="model-chip" key={`${model.name}:${model.alias ?? ""}`} title={model.name}>
          {model.alias || model.name}
        </span>
      ))}
      {hiddenCount > 0 && <span className="model-chip muted">{`+${hiddenCount}`}</span>}
    </div>
  );
}
