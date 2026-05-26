import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ScreenCenter } from "../components/screen-center";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { CopyButton } from "../components/ui/copy-button";
import { Dialog } from "../components/ui/dialog";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Toast } from "../components/ui/toast";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";

export function SetupPage() {
  const { setAuthenticated, t } = useAppContext();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const result = await api.setup(username, password);
      setToken(result.token);
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  return (
    <ScreenCenter>
      <Card className="auth-panel">
        <CardHeader>
          <CardTitle>{t("setupTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="form-stack" onSubmit={submit}>
            <Field label={t("setupUsername")}>
              <Input value={username} onChange={(event) => setUsername(event.target.value)} />
            </Field>
            <Field label={t("setupPassword")}>
              <Input
                minLength={6}
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </Field>
            {error && <Toast variant="destructive">{error}</Toast>}
            <Button disabled={Boolean(token)} type="submit">
              {t("setupSubmit")}
            </Button>
          </form>
        </CardContent>
      </Card>
      <Dialog open={Boolean(token)} title={t("setupToken")}>
        <div className="form-stack">
          <p className="hint-text">{t("tokenUsageHint")}</p>
          <code className="token-box">{token}</code>
          <CopyButton copiedLabel={t("copied")} label={t("copy")} value={token} />
          <Button
            onClick={() => {
              setAuthenticated(username);
              navigate("/dashboard");
            }}
            type="button"
          >
            {t("continue")}
          </Button>
        </div>
      </Dialog>
    </ScreenCenter>
  );
}
