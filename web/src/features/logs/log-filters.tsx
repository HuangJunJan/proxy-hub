import { Button } from "../../components/ui/button";
import { Card, CardContent } from "../../components/ui/card";
import { Field } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";

export interface LogFilters {
  channel: string;
  model: string;
  status: string;
  from: string;
  to: string;
}

export function LogFiltersCard({
  filters,
  onApply,
  onChange,
  onClear,
  onRefreshIntervalChange,
  refreshIntervalMs,
  refreshIntervalLabel,
  refreshOptionsMs,
  t,
}: {
  filters: LogFilters;
  onApply: () => void;
  onChange: (name: keyof LogFilters, value: string) => void;
  onClear: () => void;
  onRefreshIntervalChange: (value: number) => void;
  refreshIntervalMs: number;
  refreshIntervalLabel: string;
  refreshOptionsMs: readonly number[];
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
        <div className="log-filter-grid compact">
          <Field label={t("channelName")}>
            <Input value={filters.channel} onChange={(event) => onChange("channel", event.target.value)} />
          </Field>
          <Field label={t("model")}>
            <Input value={filters.model} onChange={(event) => onChange("model", event.target.value)} />
          </Field>
          <Field label={t("status")}>
            <Select value={filters.status || "all"} onValueChange={(value) => onChange("status", value === "all" ? "" : value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("all")}</SelectItem>
                <SelectItem value="success">{t("successOnly")}</SelectItem>
                <SelectItem value="error">{t("errorsOnly")}</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field label={t("autoRefresh")}>
            <Select value={String(refreshIntervalMs)} onValueChange={(value) => onRefreshIntervalChange(Number(value))}>
              <SelectTrigger>
                <SelectValue>{refreshIntervalLabel}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {refreshOptionsMs.map((option) => (
                  <SelectItem key={option} value={String(option)}>
                    {Math.round(option / 1000)}s
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
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
