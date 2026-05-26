import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ScreenCenter } from "../components/screen-center";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { Toast } from "../components/ui/toast";
import { api, getErrorMessage } from "../lib/api";
import { useAppContext } from "../lib/app-context";

export function LoginPage() {
  const { setAuthenticated, t } = useAppContext();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const result = await api.login(username, password);
      setAuthenticated(result.username);
      navigate("/dashboard");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  return (
    <ScreenCenter>
      <Card className="auth-panel">
        <CardHeader>
          <CardTitle>{t("loginTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="form-stack" onSubmit={submit}>
            <Field label={t("setupUsername")}>
              <Input value={username} onChange={(event) => setUsername(event.target.value)} />
            </Field>
            <Field label={t("setupPassword")}>
              <Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
            </Field>
            {error && <Toast variant="destructive">{error}</Toast>}
            <Button type="submit">{t("loginSubmit")}</Button>
          </form>
        </CardContent>
      </Card>
    </ScreenCenter>
  );
}
