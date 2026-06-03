import { StatTile } from "../../components/ui/stat-tile";

export function MetricCard({
  label,
  tone,
  value,
}: {
  label: string;
  tone?: "good" | "bad";
  value: number;
}) {
  return (
    <StatTile
      className="metric-tile"
      label={label}
      tone={tone === "good" ? "success" : tone === "bad" ? "danger" : undefined}
      value={value.toLocaleString()}
    />
  );
}
