import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "../components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Toast } from "../components/ui/toast";
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
  const [error, setError] = useState("");

  async function refresh() {
    try {
      setChannels(await api.channels());
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    const models = selectedModels(form, modelRows);
    if (models.length === 0) {
      setError(t("modelRequired"));
      return;
    }

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
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function probe() {
    try {
      const result = await api.probeModels(form.baseUrl, form.apiKey);
      const uniqueModels = Array.from(new Set(result.models));
      setModelRows(uniqueModels.map((name) => ({ alias: name, name, selected: true })));
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function checkHealth(name: string) {
    try {
      const result = await api.healthCheckChannel("openai-api", name);
      setHealth((current) => ({ ...current, [name]: result }));
      setError("");
    } catch (err) {
      const message = getErrorMessage(err);
      setHealth((current) => ({ ...current, [name]: { error: message, ok: false } }));
      setError(message);
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
    setModelRows(channel.models.map((model) => ({ alias: model.alias || model.name, name: model.name, selected: true })));
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
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function toggleOAuthChannel(channel: ChannelsResponse["chatgpt-oauth"][number]) {
    try {
      await api.updateChannel("chatgpt-oauth", channel.name, { ...channel, disabled: !channel.disabled });
      await refresh();
    } catch (err) {
      setError(getErrorMessage(err));
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
                {t("createChannel")}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{editingChannel ? t("editChannel") : t("createChannel")}</SheetTitle>
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
      {error && <Toast variant="destructive">{error}</Toast>}
      <Tabs
        onValueChange={(value) => {
          if (value === "openai-api" || value === "chatgpt-oauth") {
            setActiveType(value);
          }
        }}
        value={activeType}
      >
        <TabsList>
          <TabsTrigger value="openai-api">{t("openAIChannels")}</TabsTrigger>
          <TabsTrigger value="chatgpt-oauth">{t("oauthChannels")}</TabsTrigger>
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
    <form className="form-stack" onSubmit={onSubmit}>
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
      <Field label={t("priority")}>
        <Input type="number" value={form.priority} onChange={(event) => setForm({ ...form, priority: event.target.value })} />
      </Field>
      <div className="row">
        <Field label={t("manualModel")}>
          <Input value={form.model} onChange={(event) => setForm({ ...form, model: event.target.value })} />
        </Field>
        <Field label={t("alias")}>
          <Input value={form.alias} onChange={(event) => setForm({ ...form, alias: event.target.value })} />
        </Field>
      </div>
      <Button onClick={onProbe} type="button" variant="outline">
        {t("probeModels")}
      </Button>
      {modelRows.length > 0 && <ModelSelector modelRows={modelRows} setModelRows={setModelRows} t={t} />}
      <Button type="submit">{t("save")}</Button>
    </form>
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
        <strong>{t("selectVisibleModels")}</strong>
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
  const models = modelRows
    .filter((row) => row.selected)
    .map((row) => ({ alias: row.alias.trim() || undefined, name: row.name.trim() }))
    .filter((row) => row.name);

  if (form.model.trim()) {
    models.unshift({ alias: form.alias.trim() || undefined, name: form.model.trim() });
  }

  return models;
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
