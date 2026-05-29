import { Button } from "../../components/ui/button";
import { Card, CardContent } from "../../components/ui/card";
import { Field } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { Select } from "../../components/ui/select";

export interface LogFilters {
  channel: string;
  apiKey: string;
  model: string;
  endpoint: string;
  requestType: string;
  statusClass: string;
  status: string;
  errorKind: string;
  from: string;
  to: string;
}

export function LogFiltersCard({
  filters,
  onApply,
  onChange,
  onClear,
  t,
}: {
  filters: LogFilters;
  onApply: () => void;
  onChange: (name: keyof LogFilters, value: string) => void;
  onClear: () => void;
  t: (key: string) => string;
}) {
  return (
    <Card className="log-filter-card">
      <CardContent>
        <div className="log-filter-heading">
          <div>
            <h3>{t("filters")}</h3>
            <span>{t("logFilterHint")}</span>
          </div>
        </div>
        <div className="log-filter-grid">
          <Field label={t("channelName")}>
            <Input value={filters.channel} onChange={(event) => onChange("channel", event.target.value)} />
          </Field>
          <Field label={t("apiKeyShort")}>
            <Input value={filters.apiKey} onChange={(event) => onChange("apiKey", event.target.value)} />
          </Field>
          <Field label={t("model")}>
            <Input value={filters.model} onChange={(event) => onChange("model", event.target.value)} />
          </Field>
          <Field label={t("endpoint")}>
            <Input value={filters.endpoint} onChange={(event) => onChange("endpoint", event.target.value)} />
          </Field>
          <Field label={t("requestType")}>
            <Select value={filters.requestType} onChange={(event) => onChange("requestType", event.target.value)}>
              <option value="">{t("all")}</option>
              <option value="chat.completions">chat.completions</option>
              <option value="responses">responses</option>
            </Select>
          </Field>
          <Field label={t("statusGroup")}>
            <Select value={filters.statusClass} onChange={(event) => onChange("statusClass", event.target.value)}>
              <option value="">{t("all")}</option>
              <option value="success">{t("successOnly")}</option>
              <option value="error">{t("errorsOnly")}</option>
            </Select>
          </Field>
          <Field label={t("statusCode")}>
            <Input inputMode="numeric" value={filters.status} onChange={(event) => onChange("status", event.target.value)} />
          </Field>
          <Field label={t("errorKind")}>
            <Input value={filters.errorKind} onChange={(event) => onChange("errorKind", event.target.value)} />
          </Field>
          <Field label={t("fromTime")}>
            <Input type="datetime-local" value={filters.from} onChange={(event) => onChange("from", event.target.value)} />
          </Field>
          <Field label={t("toTime")}>
            <Input type="datetime-local" value={filters.to} onChange={(event) => onChange("to", event.target.value)} />
          </Field>
        </div>
        <div className="log-filter-actions">
          <Button onClick={onApply} type="button">
            {t("apply")}
          </Button>
          <Button onClick={onClear} type="button" variant="outline">
            {t("clear")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
