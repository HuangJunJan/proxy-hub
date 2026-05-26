import { Card, CardContent } from "../../components/ui/card";
import { cn } from "../../lib/cn";

export function MetricCard({ label, tone, value }: { label: string; tone?: "good" | "bad"; value: number }) {
  return (
    <Card className={cn("metric", tone)}>
      <CardContent>
        <span>{label}</span>
        <strong>{value.toLocaleString()}</strong>
      </CardContent>
    </Card>
  );
}
