import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AuthScreen } from "../components/layout/auth-screen";
import { Button } from "../components/ui/button";
import { CopyButton } from "../components/ui/copy-button";
import { Dialog } from "../components/ui/dialog";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";

export function SetupPage() {
  const { setAuthenticated, t } = useAppContext();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const result = await api.setup(username, password);
      setToken(result.token);
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  return (
    <>
      <AuthScreen description={t("tokenUsageHint")} title={t("setupTitle")}>
        <form className="form-stack" onSubmit={submit}>
          <Field label={t("setupUsername")}>
            <Input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </Field>
          <Field label={t("setupPassword")}>
            <Input
              minLength={6}
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </Field>
          <Button disabled={Boolean(token)} type="submit">
            {t("setupSubmit")}
          </Button>
        </form>
      </AuthScreen>
      <Dialog open={Boolean(token)} title={t("setupToken")}>
        <div className="form-stack">
          <p className="hint-text">{t("tokenUsageHint")}</p>
          <code className="token-box">{token}</code>
          <CopyButton
            copiedLabel={t("copied")}
            label={t("copy")}
            value={token}
          />
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
    </>
  );
}
