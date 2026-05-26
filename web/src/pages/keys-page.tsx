import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { CopyButton } from "../components/ui/copy-button";
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
  const [editingKey, setEditingKey] = useState<DownstreamKey | null>(null);
  const [error, setError] = useState("");
  const [isFormOpen, setIsFormOpen] = useState(false);
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
      if (editingKey) {
        await api.updateKey(keyIdentifier(editingKey), { name, notes });
      } else {
        const result = await api.createKey(name, notes);
        setCreated(result.token);
      }
      resetForm();
      await refresh();
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  function startCreate() {
    setEditingKey(null);
    setName("");
    setNotes("");
    setIsFormOpen(true);
  }

  function startEdit(key: DownstreamKey) {
    setEditingKey(key);
    setName(key.name || "");
    setNotes(key.notes || "");
    setIsFormOpen(true);
  }

  function resetForm() {
    setEditingKey(null);
    setName("");
    setNotes("");
    setIsFormOpen(false);
  }

  async function toggleKey(key: DownstreamKey) {
    try {
      await api.updateKey(keyIdentifier(key), { disabled: !key.disabled });
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
                {t("createKey")}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{editingKey ? t("editKey") : t("createKey")}</SheetTitle>
              </SheetHeader>
              <form className="form-stack" onSubmit={create}>
                <Field label={t("keyName")}>
                  <Input value={name} onChange={(event) => setName(event.target.value)} />
                </Field>
                <Field label={t("notes")}>
                  <Textarea value={notes} onChange={(event) => setNotes(event.target.value)} />
                </Field>
                <Button type="submit">{editingKey ? t("save") : t("createKey")}</Button>
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
              <span className={key.disabled ? "status-text danger" : "status-text success"}>
                {key.disabled ? t("disabled") : t("enabled")}
              </span>,
              <div className="table-actions" key={key.tokenMask}>
                <CopyButton copiedLabel={t("copied")} label={t("copy")} value={key.token || key.tokenMask} />
                <Button onClick={() => startEdit(key)} size="sm" type="button" variant="outline">
                  {t("edit")}
                </Button>
                <Button onClick={() => void toggleKey(key)} size="sm" type="button" variant="outline">
                  {key.disabled ? t("enable") : t("disable")}
                </Button>
              </div>,
            ])}
          />
        </CardContent>
      </Card>
      <Dialog onClose={() => setCreated("")} open={Boolean(created)} title={t("token")}>
        <div className="form-stack">
          <p className="hint-text">{t("tokenUsageHint")}</p>
          <code className="token-box">{created}</code>
          <CopyButton copiedLabel={t("copied")} label={t("copy")} value={created} />
        </div>
      </Dialog>
    </section>
  );
}

function keyIdentifier(key: DownstreamKey) {
  return key.token || key.name || key.tokenMask;
}
