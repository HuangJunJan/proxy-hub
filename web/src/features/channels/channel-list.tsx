import { Activity, Edit3, Power, Trash2 } from "lucide-react";
import { ReactNode } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { cn } from "../../lib/cn";
import type {
  ChannelHealthResult,
  ModelEntry,
  OAuthChannel,
  OpenAIChannel,
} from "../../lib/types";
import { SectionCard } from "../../components/ui/section-card";

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
    <SectionCard
      className="channel-list-card"
      description={`${safeItems.length.toLocaleString()} ${t("channels")}`}
      headerClassName="channel-list-header"
      title={title}
    >
      {safeItems.length === 0 ? (
        <div className="channel-empty-state">
          <strong>{t("noChannels")}</strong>
          <span>
            {t(type === "openai-api" ? "openAIEmptyHint" : "oauthEmptyHint")}
          </span>
        </div>
      ) : (
        <div className="channel-list">
          {safeItems.map((item) => (
            <ChannelRow
              health={health?.[item.name]}
              item={item}
              key={item.name}
              onCheckHealth={onCheckHealth}
              onDelete={onDelete}
              onEdit={onEdit}
              onToggle={onToggle}
              t={t}
              type={type}
            />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function ChannelRow({
  health,
  item,
  onCheckHealth,
  onDelete,
  onEdit,
  onToggle,
  t,
  type,
}: {
  health?: ChannelHealthResult;
  item: ChannelListItem;
  onCheckHealth?: (name: string) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
  onEdit?: (item: OpenAIChannel) => void;
  onToggle: (item: ChannelListItem) => Promise<void>;
  t: (key: string) => string;
  type: "chatgpt-oauth" | "openai-api";
}) {
  const isOpenAI = "base-url" in item;

  return (
    <article className={cn("channel-row", item.disabled && "is-disabled")}>
      <div className="channel-row-main">
        <div className="channel-title-line">
          <span
            className={cn(
              "channel-status-dot",
              item.disabled ? "is-danger" : "is-success",
            )}
            aria-hidden="true"
          />
          <div className="channel-title-copy">
            <strong>{item.name}</strong>
            <span>
              {item.notes || (isOpenAI ? item["base-url"] : t("oauthToken"))}
            </span>
          </div>
          <div className="channel-badges">
            <Badge variant={item.disabled ? "danger" : "success"}>
              {item.disabled ? t("disabled") : t("enabled")}
            </Badge>
            <Badge variant="muted">
              {isOpenAI ? t("openAIType") : t("oauthType")}
            </Badge>
          </div>
        </div>

        <div className="channel-meta-grid">
          <MetaBlock
            label={t("upstream")}
            value={<UpstreamCell item={item} t={t} />}
          />
          <MetaBlock
            label={t("credentials")}
            value={credentialLabel(item, t)}
          />
          <MetaBlock
            label={t("routing")}
            value={<RoutingCell item={item} t={t} />}
          />
          <MetaBlock
            label={t("mappingCount")}
            value={
              type === "openai-api"
                ? `${(item.models ?? []).length.toLocaleString()} ${t("mappings")}`
                : t("notApplicable")
            }
          />
        </div>

        <div className="channel-models">
          <span>{t("aliasMappings")}</span>
          <ModelList models={item.models} t={t} type={type} />
        </div>
      </div>

      <aside className="channel-row-side">
        <HealthCell
          health={health}
          name={item.name}
          onCheckHealth={onCheckHealth}
          t={t}
          type={type}
        />
        <div className="channel-actions">
          {isOpenAI && onEdit && (
            <Button
              onClick={() => onEdit(item)}
              size="sm"
              type="button"
              variant="outline"
            >
              <Edit3 size={14} />
              {t("edit")}
            </Button>
          )}
          <Button
            onClick={() => void onToggle(item)}
            size="sm"
            type="button"
            variant="outline"
          >
            <Power size={14} />
            {item.disabled ? t("enable") : t("disable")}
          </Button>
          <Button
            onClick={() => void onDelete(item.name)}
            size="sm"
            type="button"
            variant="destructive"
          >
            <Trash2 size={14} />
            {t("delete")}
          </Button>
        </div>
      </aside>
    </article>
  );
}

function MetaBlock({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="channel-meta-item">
      <span>{label}</span>
      <div>{value}</div>
    </div>
  );
}

function UpstreamCell({
  item,
  t,
}: {
  item: ChannelListItem;
  t: (key: string) => string;
}) {
  if ("base-url" in item) {
    return (
      <code className="table-code" title={item["base-url"]}>
        {item["base-url"]}
      </code>
    );
  }
  return <span className="muted-text">{t("oauthToken")}</span>;
}

function RoutingCell({
  item,
  t,
}: {
  item: ChannelListItem;
  t: (key: string) => string;
}) {
  return (
    <div className="table-cell-stack">
      {"priority" in item && (
        <span>{`${t("priority")}: ${item.priority || 100}`}</span>
      )}
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
    <div className="channel-health">
      <Badge variant={health ? (health.ok ? "success" : "danger") : "muted"}>
        {health
          ? health.ok
            ? `${t("healthy")} ${health.latencyMs ?? 0}ms`
            : health.error || t("unhealthy")
          : t("notChecked")}
      </Badge>
      <Button
        onClick={() => void onCheckHealth?.(name)}
        size="sm"
        type="button"
        variant="outline"
      >
        <Activity size={14} />
        {t("healthCheck")}
      </Button>
    </div>
  );
}

function credentialLabel(item: ChannelListItem, t: (key: string) => string) {
  if ("api-key-entries" in item) {
    return `${item["api-key-entries"]?.length ?? 0} ${t("apiKeyEntries")}`;
  }
  return item.oauth?.["refresh-token"] || item.oauth?.["access-token"]
    ? t("configured")
    : t("notConfigured");
}

function ModelList({
  models,
  t,
  type,
}: {
  models: ModelEntry[] | null | undefined;
  t: (key: string) => string;
  type: "chatgpt-oauth" | "openai-api";
}) {
  const safeModels = models ?? [];
  const visible = safeModels.slice(0, 8);
  const hiddenCount = Math.max(0, safeModels.length - visible.length);

  if (type !== "openai-api") {
    return <span className="muted-text">{t("oauthModelsUnusedHint")}</span>;
  }

  if (safeModels.length === 0) {
    return <span className="muted-text">{t("passthroughModelsHint")}</span>;
  }

  return (
    <div className="model-chip-list">
      {visible.map((model) => (
        <span
          className="model-chip"
          key={`${model.name}:${model.alias ?? ""}`}
          title={model.name}
        >
          <span>{model.alias || model.name}</span>
          {model.alias && model.alias !== model.name && (
            <small>{model.name}</small>
          )}
        </span>
      ))}
      {hiddenCount > 0 && (
        <span className="model-chip muted">{`+${hiddenCount}`}</span>
      )}
    </div>
  );
}
