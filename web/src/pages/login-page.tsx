import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AuthScreen } from "../components/layout/auth-screen";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";

export function LoginPage() {
  const { setAuthenticated, t } = useAppContext();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const result = await api.login(username, password);
      setAuthenticated(result.username);
      navigate("/dashboard");
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  return (
    <AuthScreen title={t("loginTitle")}>
      <form className="form-stack" onSubmit={submit}>
        <Field label={t("setupUsername")}>
          <Input
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
        </Field>
        <Field label={t("setupPassword")}>
          <Input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>
        <Button type="submit">{t("loginSubmit")}</Button>
      </form>
    </AuthScreen>
  );
}
