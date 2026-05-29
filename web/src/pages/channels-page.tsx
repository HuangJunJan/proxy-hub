import { KeyRound, Plus, Route, ShieldCheck } from "lucide-react";
import { FormEvent, ReactNode, useEffect, useMemo, useState } from "react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "../components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Toolbar } from "../components/ui/toolbar";
import { ChannelList } from "../features/channels/channel-list";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { ChannelHealthResult, ChannelsResponse, ModelEntry, OAuthChannel, OpenAIChannel } from "../lib/types";

type ChannelFormState = { alias: string; apiKey: string; baseUrl: string; model: string; name: string; priority: string };
type ModelSelection = { alias: string; name: string; selected: boolean };

const emptyChannels: ChannelsResponse = { "chatgpt-oauth": [], "openai-api": [] };
const emptyForm: ChannelFormState = { alias: "", apiKey: "", baseUrl: "", model: "", name: "", priority: "100" };

export function ChannelsPage() {
  const { t } = useAppContext();
  const [activeType, setActiveType] = useState<"chatgpt-oauth" | "openai-api">("openai-api");
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [form, setForm] = useState<ChannelFormState>(emptyForm);
  const [health, setHealth] = useState<Record<string, ChannelHealthResult | undefined>>({});
  const [editingChannel, setEditingChannel] = useState<OpenAIChannel | null>(null);
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
    const mappingError = aliasMappingError(form);
    if (mappingError) {
      throw new Error(t(mappingError));
    }
    const models = selectedModels(form, modelRows);

    const channel: OpenAIChannel = {
      ...(editingChannel ?? {}),
      "api-key-entries": apiKeyEntriesForSave(editingChannel, form.apiKey),
      "base-url": form.baseUrl,
      models,
      name: form.name,
      priority: Number(form.priority || 100),
    };
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
    try {
      const result = await api.probeModels(form.baseUrl, form.apiKey);
      const uniqueModels = Array.from(new Set(result.models));
      setModelRows((current) => {
        const existing = new Map(current.map((row) => [row.name, row]));
        return uniqueModels.map((name) => existing.get(name) ?? { alias: "", name, selected: false });
      });
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  async function checkHealth(name: string) {
    try {
      const result = await api.healthCheckChannel("openai-api", name);
      setHealth((current) => ({ ...current, [name]: result }));
    } catch (err) {
      const message = getErrorMessage(err);
      setHealth((current) => ({ ...current, [name]: { error: message, ok: false } }));
    }
  }

  function startCreate() {
    setEditingChannel(null);
    setForm(emptyForm);
    setModelRows([]);
    setIsFormOpen(true);
  }

  function startEdit(channel: OpenAIChannel) {
    setEditingChannel(channel);
    setForm({
      alias: "",
      apiKey: channel["api-key-entries"]?.[0]?.["api-key"] ?? "",
      baseUrl: channel["base-url"],
      model: "",
      name: channel.name,
      priority: String(channel.priority || 100),
    });
    setModelRows((channel.models ?? []).map((model) => ({ alias: model.alias ?? "", name: model.name, selected: true })));
    setIsFormOpen(true);
  }

  function resetForm() {
    setEditingChannel(null);
    setForm(emptyForm);
    setIsFormOpen(false);
    setModelRows([]);
  }

  async function toggleOpenAIChannel(channel: OpenAIChannel | OAuthChannel) {
    if (!("base-url" in channel)) {
      return;
    }
    try {
      await api.updateChannel("openai-api", channel.name, { ...channel, disabled: !channel.disabled });
      await refresh();
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  async function toggleOAuthChannel(channel: ChannelsResponse["chatgpt-oauth"][number]) {
    try {
      await api.updateChannel("chatgpt-oauth", channel.name, { ...channel, disabled: !channel.disabled });
      await refresh();
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  return (
    <section className="stack">
      <Toolbar
        actions={
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
                <SheetTitle>{editingChannel ? t("editChannel") : t("createChannel")}</SheetTitle>
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
        }
        onRefresh={refresh}
        refreshLabel={t("refresh")}
        title={t("channels")}
      />
      <div className="channel-summary-grid">
        <SummaryTile icon={<Route size={16} />} label={t("totalChannels")} value={summary.total} />
        <SummaryTile icon={<ShieldCheck size={16} />} label={t("enabled")} tone="success" value={summary.enabled} />
        <SummaryTile icon={<KeyRound size={16} />} label={t("credentials")} value={summary.credentials} />
        <SummaryTile icon={<Route size={16} />} label={t("mappingCount")} value={summary.mappings} />
      </div>
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
            <Badge variant="muted">{channels["chatgpt-oauth"].length}</Badge>
          </TabsTrigger>
        </TabsList>
        <TabsContent value="openai-api">
          <ChannelList
            health={health}
            items={channels["openai-api"]}
            onCheckHealth={checkHealth}
            onDelete={async (name) => {
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
    </section>
  );
}

function ChannelCreateForm({
  form,
  modelRows,
  onProbe,
  onSubmit,
  setForm,
  setModelRows,
  t,
}: {
  form: ChannelFormState;
  modelRows: ModelSelection[];
  onProbe: () => void;
  onSubmit: (event: FormEvent) => void;
  setForm: (form: ChannelFormState) => void;
  setModelRows: (rows: ModelSelection[]) => void;
  t: (key: string) => string;
}) {
  return (
    <form className="channel-form" onSubmit={onSubmit}>
      <FormSection description={t("channelConnectionHint")} title={t("connection")}>
        <Field label={t("channelName")}>
          <Input required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
        </Field>
        <Field label={t("baseUrl")}>
          <Input required value={form.baseUrl} onChange={(event) => setForm({ ...form, baseUrl: event.target.value })} />
        </Field>
        <Field label={t("apiKey")}>
          <Input
            required
            type="password"
            value={form.apiKey}
            onChange={(event) => setForm({ ...form, apiKey: event.target.value })}
          />
        </Field>
      </FormSection>

      <FormSection description={t("channelRoutingHint")} title={t("routing")}>
        <div className="channel-form-two-col">
          <Field label={t("priority")}>
            <Input
              min={0}
              type="number"
              value={form.priority}
              onChange={(event) => setForm({ ...form, priority: event.target.value })}
            />
          </Field>
          <div className="channel-priority-note">
            <strong>{t("lowerPriorityWins")}</strong>
            <span>{t("priorityHint")}</span>
          </div>
        </div>
      </FormSection>

      <FormSection description={t("channelModelsHint")} title={t("aliasMappings")}>
        <div className="channel-manual-model">
          <Field label={t("modelName")}>
            <Input value={form.model} onChange={(event) => setForm({ ...form, model: event.target.value })} />
          </Field>
          <Field label={t("alias")}>
            <Input placeholder={t("optionalAlias")} value={form.alias} onChange={(event) => setForm({ ...form, alias: event.target.value })} />
          </Field>
        </div>
        <div className="channel-probe-row">
          <Button disabled={!form.baseUrl.trim() || !form.apiKey.trim()} onClick={onProbe} type="button" variant="outline">
            {t("probeModels")}
          </Button>
          <span>{t("probeModelsHint")}</span>
        </div>
        {modelRows.length > 0 && <ModelSelector modelRows={modelRows} setModelRows={setModelRows} t={t} />}
      </FormSection>

      <div className="channel-form-footer">
        <Button type="submit">{t("save")}</Button>
      </div>
    </form>
  );
}

function FormSection({ children, description, title }: { children: ReactNode; description: string; title: string }) {
  return (
    <section className="channel-form-section">
      <div className="channel-form-section-heading">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
      <div className="channel-form-section-body">{children}</div>
    </section>
  );
}

function ModelSelector({
  modelRows,
  setModelRows,
  t,
}: {
  modelRows: ModelSelection[];
  setModelRows: (rows: ModelSelection[]) => void;
  t: (key: string) => string;
}) {
  return (
    <div className="model-selector">
      <div className="model-selector-header">
        <strong>{t("aliasMappingCandidates")}</strong>
        <span>{`${modelRows.filter((row) => row.selected).length}/${modelRows.length}`}</span>
      </div>
      <div className="model-selector-list">
        {modelRows.map((row, index) => (
          <label className="model-selector-row" key={row.name}>
            <input
              checked={row.selected}
              onChange={(event) => {
                const next = [...modelRows];
                next[index] = { ...row, selected: event.target.checked };
                setModelRows(next);
              }}
              type="checkbox"
            />
            <code title={row.name}>{row.name}</code>
            <Input
              aria-label={`${t("alias")} ${row.name}`}
              placeholder={t("alias")}
              value={row.alias}
              onChange={(event) => {
                const next = [...modelRows];
                next[index] = { ...row, alias: event.target.value };
                setModelRows(next);
              }}
            />
          </label>
        ))}
      </div>
    </div>
  );
}

function selectedModels(form: ChannelFormState, modelRows: ModelSelection[]): ModelEntry[] {
  const models: ModelEntry[] = [];
  const seen = new Set<string>();

  pushModelEntry(models, seen, form.model, form.alias);
  for (const row of modelRows) {
    if (row.selected) {
      pushModelEntry(models, seen, row.name, row.alias);
    }
  }

  return models;
}

function aliasMappingError(form: ChannelFormState) {
  const hasManualModel = !!form.model.trim();
  const hasManualAlias = !!form.alias.trim();
  if (!hasManualModel && hasManualAlias) {
    return "aliasRequiredForMapping";
  }
  return null;
}

function pushModelEntry(models: ModelEntry[], seen: Set<string>, rawName: string, rawAlias: string) {
  const name = rawName.trim();
  const alias = rawAlias.trim();
  if (!name) {
    return;
  }
  const normalizedAlias = alias && alias !== name ? alias : "";
  const key = `${name}\x00${normalizedAlias}`;
  if (seen.has(key)) {
    return;
  }
  seen.add(key);
  models.push(normalizedAlias ? { alias: normalizedAlias, name } : { name });
}

function apiKeyEntriesForSave(editingChannel: OpenAIChannel | null, apiKey: string): OpenAIChannel["api-key-entries"] {
  const entries = editingChannel?.["api-key-entries"] ? [...editingChannel["api-key-entries"]] : [];
  const trimmed = apiKey.trim();
  if (trimmed) {
    if (entries.length > 0) {
      entries[0] = { ...entries[0], "api-key": trimmed };
    } else {
      entries.push({ "api-key": trimmed });
    }
  }
  return entries;
}

function channelSummary(channels: ChannelsResponse) {
  const all = [...channels["openai-api"], ...channels["chatgpt-oauth"]];
  return {
    credentials: channels["openai-api"].reduce((total, channel) => total + (channel["api-key-entries"]?.length ?? 0), 0),
    enabled: all.filter((channel) => !channel.disabled).length,
    mappings: channels["openai-api"].reduce((total, channel) => total + (channel.models?.length ?? 0), 0),
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
    <Card className="channel-summary-tile">
      <CardContent>
        <span className={tone === "success" ? "channel-summary-icon is-success" : "channel-summary-icon"}>{icon}</span>
        <div>
          <span>{label}</span>
          <strong>{value.toLocaleString()}</strong>
        </div>
      </CardContent>
    </Card>
  );
}
