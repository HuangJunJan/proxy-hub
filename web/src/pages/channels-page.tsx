import { KeyRound, Plus, Route, ShieldCheck } from "lucide-react";
import { FormEvent, ReactNode, useEffect, useMemo, useState } from "react";
import {
  Page,
  PageBody,
  PageDescription,
  PageHeader,
  PageTitle,
} from "../components/layout/page";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { PageActions, RefreshButton } from "../components/ui/page-actions";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "../components/ui/sheet";
import { StatTile } from "../components/ui/stat-tile";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "../components/ui/tabs";
import { useConfirm } from "../components/ui/use-confirm";
import {
  channelAliasMappingError,
  channelAPIKeyEntriesForSave,
  ChannelCreateForm,
  channelFormFromChannel,
  channelModelsForSave,
  emptyChannelForm,
  type ChannelFormState,
  type ModelSelection,
} from "../features/channels/channel-form";
import { ChannelList } from "../features/channels/channel-list";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type {
  ChannelHealthResult,
  ChannelsResponse,
  OAuthChannel,
  OpenAIChannel,
} from "../lib/types";

const emptyChannels: ChannelsResponse = {
  "chatgpt-oauth": [],
  "openai-api": [],
};

export function ChannelsPage() {
  const { t } = useAppContext();
  const { confirm, confirmDialog } = useConfirm();
  const [activeType, setActiveType] = useState<"chatgpt-oauth" | "openai-api">(
    "openai-api",
  );
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [form, setForm] = useState<ChannelFormState>(emptyChannelForm);
  const [health, setHealth] = useState<
    Record<string, ChannelHealthResult | undefined>
  >({});
  const [editingChannel, setEditingChannel] = useState<OpenAIChannel | null>(
    null,
  );
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [modelRows, setModelRows] = useState<ModelSelection[]>([]);
  const summary = useMemo(() => channelSummary(channels), [channels]);

  async function refresh() {
    try {
      setChannels(await api.channels());
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    const mappingError = channelAliasMappingError(form);
    if (mappingError) {
      throw new Error(t(mappingError));
    }

    const channel: OpenAIChannel = {
      ...(editingChannel ?? {}),
      "api-key-entries": channelAPIKeyEntriesForSave(editingChannel, form.apiKey),
      "base-url": form.baseUrl,
      models: channelModelsForSave(form, modelRows),
      name: form.name,
      priority: Number(form.priority || 100),
    };

    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("save"),
      description: editingChannel
        ? t("confirmUpdateChannel")
        : t("confirmCreateChannel"),
      title: editingChannel
        ? t("confirmUpdateChannelTitle")
        : t("confirmCreateChannelTitle"),
    });
    if (!confirmed) {
      return;
    }

    try {
      if (editingChannel) {
        await api.updateChannel("openai-api", editingChannel.name, channel);
      } else {
        await api.createOpenAIChannel(channel);
      }
      resetForm();
      await refresh();
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  async function probe() {
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("probeModels"),
      description: t("confirmProbeModels"),
      title: t("confirmProbeModelsTitle"),
    });
    if (!confirmed) {
      return;
    }

    try {
      const result = await api.probeModels(form.baseUrl, form.apiKey);
      const uniqueModels = Array.from(new Set(result.models));
      setModelRows((current) => {
        const existing = new Map(current.map((row) => [row.name, row]));
        return uniqueModels.map(
          (name) => existing.get(name) ?? { alias: "", name, selected: false },
        );
      });
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  async function checkHealth(name: string) {
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("healthCheck"),
      description: t("confirmHealthCheck"),
      title: t("confirmHealthCheckTitle"),
    });
    if (!confirmed) {
      return;
    }

    try {
      const result = await api.healthCheckChannel("openai-api", name);
      setHealth((current) => ({ ...current, [name]: result }));
    } catch (err) {
      const message = getErrorMessage(err);
      setHealth((current) => ({
        ...current,
        [name]: { error: message, ok: false },
      }));
    }
  }

  function startCreate() {
    setEditingChannel(null);
    setForm(emptyChannelForm);
    setModelRows([]);
    setIsFormOpen(true);
  }

  function startEdit(channel: OpenAIChannel) {
    setEditingChannel(channel);
    const next = channelFormFromChannel(channel);
    setForm(next.form);
    setModelRows(next.modelRows);
    setIsFormOpen(true);
  }

  function resetForm() {
    setEditingChannel(null);
    setForm(emptyChannelForm);
    setIsFormOpen(false);
    setModelRows([]);
  }

  async function toggleOpenAIChannel(channel: OpenAIChannel | OAuthChannel) {
    if (!("base-url" in channel)) {
      return;
    }
    const willDisable = !channel.disabled;
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: willDisable ? t("disable") : t("enable"),
      description: willDisable
        ? t("confirmDisableChannel")
        : t("confirmEnableChannel"),
      title: willDisable
        ? t("confirmDisableChannelTitle")
        : t("confirmEnableChannelTitle"),
      tone: willDisable ? "destructive" : "default",
    });
    if (!confirmed) {
      return;
    }
    try {
      await api.updateChannel("openai-api", channel.name, {
        ...channel,
        disabled: willDisable,
      });
      await refresh();
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  async function toggleOAuthChannel(
    channel: ChannelsResponse["chatgpt-oauth"][number],
  ) {
    const willDisable = !channel.disabled;
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: willDisable ? t("disable") : t("enable"),
      description: willDisable
        ? t("confirmDisableChannel")
        : t("confirmEnableChannel"),
      title: willDisable
        ? t("confirmDisableChannelTitle")
        : t("confirmEnableChannelTitle"),
      tone: willDisable ? "destructive" : "default",
    });
    if (!confirmed) {
      return;
    }
    try {
      await api.updateChannel("chatgpt-oauth", channel.name, {
        ...channel,
        disabled: willDisable,
      });
      await refresh();
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  return (
    <Page>
      <PageHeader
        actions={
          <PageActions>
            <Sheet
              onOpenChange={(open) => {
                if (!open) {
                  resetForm();
                }
              }}
              open={isFormOpen}
            >
              <SheetTrigger asChild>
                <Button onClick={startCreate} type="button">
                  <Plus size={16} />
                  {t("createOpenAIChannel")}
                </Button>
              </SheetTrigger>
              <SheetContent className="channel-sheet">
                <SheetHeader>
                  <SheetTitle>
                    {editingChannel ? t("editChannel") : t("createChannel")}
                  </SheetTitle>
                  <p>{t("channelFormHint")}</p>
                </SheetHeader>
                <ChannelCreateForm
                  form={form}
                  modelRows={modelRows}
                  onProbe={probe}
                  onSubmit={save}
                  setForm={setForm}
                  setModelRows={setModelRows}
                  t={t}
                />
              </SheetContent>
            </Sheet>
            <RefreshButton label={t("refresh")} onClick={refresh} />
          </PageActions>
        }
      >
        <PageTitle visuallyHidden>{t("channels")}</PageTitle>
        <PageDescription>{t("openAIChannels")}</PageDescription>
      </PageHeader>
      <PageBody>
        <div className="channel-summary-grid">
          <SummaryTile
            icon={<Route size={16} />}
            label={t("totalChannels")}
            value={summary.total}
          />
          <SummaryTile
            icon={<ShieldCheck size={16} />}
            label={t("enabled")}
            tone="success"
            value={summary.enabled}
          />
          <SummaryTile
            icon={<KeyRound size={16} />}
            label={t("credentials")}
            value={summary.credentials}
          />
          <SummaryTile
            icon={<Route size={16} />}
            label={t("mappingCount")}
            value={summary.mappings}
          />
        </div>
        {confirmDialog}
        <Tabs
          onValueChange={(value) => {
            if (value === "openai-api" || value === "chatgpt-oauth") {
              setActiveType(value);
            }
          }}
          value={activeType}
        >
          <TabsList>
            <TabsTrigger value="openai-api">
              <span>{t("openAIChannels")}</span>
              <Badge variant="muted">{channels["openai-api"].length}</Badge>
            </TabsTrigger>
            <TabsTrigger value="chatgpt-oauth">
              <span>{t("oauthChannels")}</span>
              <Badge variant="muted">
                {channels["chatgpt-oauth"].length}
              </Badge>
            </TabsTrigger>
          </TabsList>
          <TabsContent value="openai-api">
            <ChannelList
              health={health}
              items={channels["openai-api"]}
              onCheckHealth={checkHealth}
              onDelete={async (name) => {
                const confirmed = await confirm({
                  cancelLabel: t("cancel"),
                  confirmLabel: t("delete"),
                  description: t("confirmDeleteChannel"),
                  title: t("confirmDeleteChannelTitle"),
                  tone: "destructive",
                });
                if (!confirmed) {
                  return;
                }
                await api.deleteChannel("openai-api", name);
                await refresh();
              }}
              onEdit={startEdit}
              onToggle={toggleOpenAIChannel}
              t={t}
              title={t("openAIChannels")}
              type="openai-api"
            />
          </TabsContent>
          <TabsContent value="chatgpt-oauth">
            <ChannelList
              items={channels["chatgpt-oauth"]}
              onDelete={async (name) => {
                const confirmed = await confirm({
                  cancelLabel: t("cancel"),
                  confirmLabel: t("delete"),
                  description: t("confirmDeleteChannel"),
                  title: t("confirmDeleteChannelTitle"),
                  tone: "destructive",
                });
                if (!confirmed) {
                  return;
                }
                await api.deleteChannel("chatgpt-oauth", name);
                await refresh();
              }}
              onToggle={toggleOAuthChannel}
              t={t}
              title={t("oauthChannels")}
              type="chatgpt-oauth"
            />
          </TabsContent>
        </Tabs>
      </PageBody>
    </Page>
  );
}

function channelSummary(channels: ChannelsResponse) {
  const all = [...channels["openai-api"], ...channels["chatgpt-oauth"]];
  return {
    credentials: channels["openai-api"].reduce(
      (total, channel) => total + (channel["api-key-entries"]?.length ?? 0),
      0,
    ),
    enabled: all.filter((channel) => !channel.disabled).length,
    mappings: channels["openai-api"].reduce(
      (total, channel) => total + (channel.models?.length ?? 0),
      0,
    ),
    total: all.length,
  };
}

function SummaryTile({
  icon,
  label,
  tone,
  value,
}: {
  icon: ReactNode;
  label: string;
  tone?: "success";
  value: number;
}) {
  return (
    <StatTile
      className="channel-summary-tile"
      icon={icon}
      iconTone={tone}
      label={label}
      value={value.toLocaleString()}
    />
  );
}
