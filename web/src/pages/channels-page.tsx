import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { Badge } from "../components/ui/badge";
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
import type { ChannelsResponse, ModelEntry, OpenAIChannel } from "../lib/types";

const emptyChannels: ChannelsResponse = { "chatgpt-oauth": [], "openai-api": [] };

export function ChannelsPage() {
  const { t } = useAppContext();
  const [activeType, setActiveType] = useState<"chatgpt-oauth" | "openai-api">("openai-api");
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [form, setForm] = useState({ alias: "", apiKey: "", baseUrl: "", model: "", name: "", priority: "100" });
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [probed, setProbed] = useState<string[]>([]);
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

  async function create(event: FormEvent) {
    event.preventDefault();
    const channel: OpenAIChannel = {
      "api-key-entries": [{ "api-key": form.apiKey }],
      "base-url": form.baseUrl,
      models: selectedModels(form.model, form.alias, probed),
      name: form.name,
      priority: Number(form.priority || 100),
    };
    try {
      await api.createOpenAIChannel(channel);
      setForm({ alias: "", apiKey: "", baseUrl: "", model: "", name: "", priority: "100" });
      setIsCreateOpen(false);
      setProbed([]);
      await refresh();
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function probe() {
    try {
      const result = await api.probeModels(form.baseUrl, form.apiKey);
      setProbed(result.models);
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  return (
    <section className="stack">
      <Toolbar
        actions={
          <Sheet onOpenChange={setIsCreateOpen} open={isCreateOpen}>
            <SheetTrigger asChild>
              <Button type="button">
                <Plus size={16} />
                {t("createChannel")}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{t("createChannel")}</SheetTitle>
              </SheetHeader>
              <ChannelCreateForm
                form={form}
                onProbe={probe}
                onSubmit={create}
                probed={probed}
                setForm={setForm}
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
          <TabsTrigger value="openai-api">
            {t("openAIChannels")}
          </TabsTrigger>
          <TabsTrigger value="chatgpt-oauth">
            {t("oauthChannels")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="openai-api">
          <ChannelList
            items={channels["openai-api"]}
            onDelete={async (name) => {
              await api.deleteChannel("openai-api", name);
              await refresh();
            }}
            t={t}
            title={t("openAIChannels")}
          />
        </TabsContent>
        <TabsContent value="chatgpt-oauth">
          <ChannelList
            items={channels["chatgpt-oauth"]}
            onDelete={async (name) => {
              await api.deleteChannel("chatgpt-oauth", name);
              await refresh();
            }}
            t={t}
            title={t("oauthChannels")}
          />
        </TabsContent>
      </Tabs>
    </section>
  );
}

function ChannelCreateForm({
  form,
  onProbe,
  onSubmit,
  probed,
  setForm,
  t,
}: {
  form: { alias: string; apiKey: string; baseUrl: string; model: string; name: string; priority: string };
  onProbe: () => void;
  onSubmit: (event: FormEvent) => void;
  probed: string[];
  setForm: (form: { alias: string; apiKey: string; baseUrl: string; model: string; name: string; priority: string }) => void;
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
        <Field label={t("modelName")}>
          <Input value={form.model} onChange={(event) => setForm({ ...form, model: event.target.value })} />
        </Field>
        <Field label={t("alias")}>
          <Input value={form.alias} onChange={(event) => setForm({ ...form, alias: event.target.value })} />
        </Field>
      </div>
      <Button onClick={onProbe} type="button" variant="outline">
        {t("probeModels")}
      </Button>
      {probed.length > 0 && (
        <div className="chip-list">
          {probed.slice(0, 12).map((model) => (
            <Badge key={model} variant="muted">
              {model}
            </Badge>
          ))}
        </div>
      )}
      <Button type="submit">{t("save")}</Button>
    </form>
  );
}

function selectedModels(model: string, alias: string, probed: string[]): ModelEntry[] {
  if (model.trim()) {
    return [{ alias: alias.trim() || undefined, name: model.trim() }];
  }
  return probed.slice(0, 20).map((name) => ({ name }));
}
