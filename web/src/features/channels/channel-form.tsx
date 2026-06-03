import type { FormEvent, ReactNode } from "react";
import { Button } from "../../components/ui/button";
import { Field } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import type { ModelEntry, OpenAIChannel } from "../../lib/types";

export type ChannelFormState = {
  alias: string;
  apiKey: string;
  baseUrl: string;
  model: string;
  name: string;
  priority: string;
};

export type ModelSelection = {
  alias: string;
  name: string;
  selected: boolean;
};

export const emptyChannelForm: ChannelFormState = {
  alias: "",
  apiKey: "",
  baseUrl: "",
  model: "",
  name: "",
  priority: "100",
};

export function ChannelCreateForm({
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
      <FormSection
        description={t("channelConnectionHint")}
        title={t("connection")}
      >
        <Field label={t("channelName")}>
          <Input
            required
            value={form.name}
            onChange={(event) => setForm({ ...form, name: event.target.value })}
          />
        </Field>
        <Field label={t("baseUrl")}>
          <Input
            required
            value={form.baseUrl}
            onChange={(event) =>
              setForm({ ...form, baseUrl: event.target.value })
            }
          />
        </Field>
        <Field label={t("apiKey")}>
          <Input
            required
            type="password"
            value={form.apiKey}
            onChange={(event) =>
              setForm({ ...form, apiKey: event.target.value })
            }
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
              onChange={(event) =>
                setForm({ ...form, priority: event.target.value })
              }
            />
          </Field>
          <div className="channel-priority-note">
            <strong>{t("lowerPriorityWins")}</strong>
            <span>{t("priorityHint")}</span>
          </div>
        </div>
      </FormSection>

      <FormSection
        description={t("channelModelsHint")}
        title={t("aliasMappings")}
      >
        <div className="channel-manual-model">
          <Field label={t("modelName")}>
            <Input
              value={form.model}
              onChange={(event) =>
                setForm({ ...form, model: event.target.value })
              }
            />
          </Field>
          <Field label={t("alias")}>
            <Input
              placeholder={t("optionalAlias")}
              value={form.alias}
              onChange={(event) =>
                setForm({ ...form, alias: event.target.value })
              }
            />
          </Field>
        </div>
        <div className="channel-probe-row">
          <Button
            disabled={!form.baseUrl.trim() || !form.apiKey.trim()}
            onClick={onProbe}
            type="button"
            variant="outline"
          >
            {t("probeModels")}
          </Button>
          <span>{t("probeModelsHint")}</span>
        </div>
        {modelRows.length > 0 ? (
          <ModelSelector
            modelRows={modelRows}
            setModelRows={setModelRows}
            t={t}
          />
        ) : null}
      </FormSection>

      <div className="channel-form-footer">
        <Button type="submit">{t("save")}</Button>
      </div>
    </form>
  );
}

export function channelFormFromChannel(channel: OpenAIChannel) {
  return {
    form: {
      alias: "",
      apiKey: channel["api-key-entries"]?.[0]?.["api-key"] ?? "",
      baseUrl: channel["base-url"],
      model: "",
      name: channel.name,
      priority: String(channel.priority || 100),
    } satisfies ChannelFormState,
    modelRows: (channel.models ?? []).map((model) => ({
      alias: model.alias ?? "",
      name: model.name,
      selected: true,
    })),
  };
}

export function channelAliasMappingError(form: ChannelFormState) {
  const hasManualModel = !!form.model.trim();
  const hasManualAlias = !!form.alias.trim();
  if (!hasManualModel && hasManualAlias) {
    return "aliasRequiredForMapping";
  }
  return null;
}

export function channelModelsForSave(
  form: ChannelFormState,
  modelRows: ModelSelection[],
): ModelEntry[] {
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

export function channelAPIKeyEntriesForSave(
  editingChannel: OpenAIChannel | null,
  apiKey: string,
): OpenAIChannel["api-key-entries"] {
  const entries = editingChannel?.["api-key-entries"]
    ? [...editingChannel["api-key-entries"]]
    : [];
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

function FormSection({
  children,
  description,
  title,
}: {
  children: ReactNode;
  description: string;
  title: string;
}) {
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
        <span>
          {`${modelRows.filter((row) => row.selected).length}/${modelRows.length}`}
        </span>
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

function pushModelEntry(
  models: ModelEntry[],
  seen: Set<string>,
  rawName: string,
  rawAlias: string,
) {
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
