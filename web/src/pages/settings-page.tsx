import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { CopyButton } from "../components/ui/copy-button";
import { Field } from "../components/ui/field";
import { Select } from "../components/ui/select";
import { useAppContext } from "../lib/app-context";
import type { Language, ThemeMode } from "../lib/types";

export function SettingsPage() {
  const { language, setLanguage, setTheme, t, theme } = useAppContext();

  return (
    <section className="settings-grid">
      <AccessPanel t={t} />
      <Card>
        <CardHeader>
          <CardTitle>{t("language")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Field label={t("language")}>
            <Select value={language} onChange={(event) => setLanguage(event.target.value as Language)}>
              <option value="zh">中文</option>
              <option value="en">English</option>
            </Select>
          </Field>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("theme")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Field label={t("theme")}>
            <Select value={theme} onChange={(event) => setTheme(event.target.value as ThemeMode)}>
              <option value="system">{t("system")}</option>
              <option value="light">{t("light")}</option>
              <option value="dark">{t("dark")}</option>
            </Select>
          </Field>
        </CardContent>
      </Card>
    </section>
  );
}

function AccessPanel({ t }: { t: (key: string) => string }) {
  const baseUrl = `${gatewayOrigin()}/v1`;
  const chatUrl = `${baseUrl}/chat/completions`;
  const modelsUrl = `${baseUrl}/models`;
  const curl = `curl ${chatUrl} -H "Authorization: Bearer <proxy-hub-api-key>" -H "Content-Type: application/json" -d "{\\"model\\":\\"<visible-model>\\",\\"messages\\":[{\\"role\\":\\"user\\",\\"content\\":\\"hi\\"}]}"`;

  return (
    <Card className="settings-wide-card">
      <CardHeader>
        <CardTitle>{t("externalAccess")}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="endpoint-grid">
          <CopyValue copiedLabel={t("copied")} label={t("openAIBaseUrl")} value={baseUrl} t={t} />
          <CopyValue copiedLabel={t("copied")} label={t("chatCompletionsUrl")} value={chatUrl} t={t} />
          <CopyValue copiedLabel={t("copied")} label={t("modelsUrl")} value={modelsUrl} t={t} />
          <CopyValue copiedLabel={t("copied")} label={t("curlExample")} value={curl} t={t} />
        </div>
      </CardContent>
    </Card>
  );
}

function CopyValue({
  copiedLabel,
  label,
  t,
  value,
}: {
  copiedLabel: string;
  label: string;
  t: (key: string) => string;
  value: string;
}) {
  return (
    <div className="copy-value">
      <span>{label}</span>
      <code>{value}</code>
      <CopyButton copiedLabel={copiedLabel} label={t("copy")} value={value} />
    </div>
  );
}

function gatewayOrigin() {
  const configuredTarget = import.meta.env.VITE_PROXY_TARGET;
  if (configuredTarget) {
    return trimTrailingSlash(configuredTarget);
  }
  if (import.meta.env.DEV) {
    return "http://localhost:8787";
  }
  return window.location.origin;
}

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}
