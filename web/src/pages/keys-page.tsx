import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { DataTable } from "../components/ui/data-table";
import { Dialog } from "../components/ui/dialog";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "../components/ui/sheet";
import { Textarea } from "../components/ui/textarea";
import { Toast } from "../components/ui/toast";
import { Toolbar } from "../components/ui/toolbar";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import type { DownstreamKey } from "../lib/types";

export function KeysPage() {
  const { t } = useAppContext();
  const [created, setCreated] = useState("");
  const [error, setError] = useState("");
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [keys, setKeys] = useState<DownstreamKey[]>([]);
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");

  async function refresh() {
    try {
      setKeys(await api.keys());
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
    try {
      const result = await api.createKey(name, notes);
      setCreated(result.token);
      setName("");
      setNotes("");
      setIsCreateOpen(false);
      await refresh();
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
                {t("createKey")}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{t("createKey")}</SheetTitle>
              </SheetHeader>
              <form className="form-stack" onSubmit={create}>
                <Field label={t("keyName")}>
                  <Input value={name} onChange={(event) => setName(event.target.value)} />
                </Field>
                <Field label={t("notes")}>
                  <Textarea value={notes} onChange={(event) => setNotes(event.target.value)} />
                </Field>
                <Button type="submit">{t("createKey")}</Button>
              </form>
            </SheetContent>
          </Sheet>
        }
        onRefresh={refresh}
        refreshLabel={t("refresh")}
        title={t("keys")}
      />
      {error && <Toast variant="destructive">{error}</Toast>}
      <Card>
        <CardContent>
          <DataTable
            empty={t("empty")}
            headers={[t("keyName"), t("token"), t("notes"), t("status"), ""]}
            rows={keys.map((key) => [
              key.name || "-",
              key.tokenMask,
              key.notes || "-",
              key.disabled ? t("disabled") : t("enabled"),
              <Button
                key={key.tokenMask}
                onClick={async () => {
                  await api.updateKey(key.name || key.tokenMask, { disabled: !key.disabled });
                  await refresh();
                }}
                size="sm"
                type="button"
                variant="outline"
              >
                {key.disabled ? t("enabled") : t("disabled")}
              </Button>,
            ])}
          />
        </CardContent>
      </Card>
      <Dialog onClose={() => setCreated("")} open={Boolean(created)} title={t("token")}>
        <code className="token-box">{created}</code>
      </Dialog>
    </section>
  );
}
