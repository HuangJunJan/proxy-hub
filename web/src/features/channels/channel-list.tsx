import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { DataTable } from "../../components/ui/data-table";
import type { ModelEntry, OAuthChannel, OpenAIChannel } from "../../lib/types";

type ChannelListItem = Pick<OpenAIChannel | OAuthChannel, "disabled" | "models" | "name">;

export function ChannelList({
  items,
  onDelete,
  title,
  t,
}: {
  items: ChannelListItem[];
  onDelete: (name: string) => Promise<void>;
  title: string;
  t: (key: string) => string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable
          empty={t("empty")}
          headers={[t("channelName"), t("status"), t("model"), ""]}
          rows={items.map((item) => [
            item.name,
            item.disabled ? t("disabled") : t("enabled"),
            modelLabels(item.models),
            <Button key={item.name} onClick={() => void onDelete(item.name)} type="button" variant="destructive">
              {t("delete")}
            </Button>,
          ])}
        />
      </CardContent>
    </Card>
  );
}

function modelLabels(models: ModelEntry[]) {
  return models.map((model) => model.alias || model.name).join(", ");
}
