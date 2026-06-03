import {
  Page,
  PageBody,
  PageDescription,
  PageHeader,
  PageTitle,
} from "../components/layout/page";
import { CopyButton } from "../components/ui/copy-button";
import { Field } from "../components/ui/field";
import { SectionCard } from "../components/ui/section-card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { useConfirm } from "../components/ui/use-confirm";
import { useAppContext } from "../lib/app-context";
import type { Language, ThemeMode } from "../lib/types";

export function SettingsPage() {
  const { language, setLanguage, setTheme, t, theme } = useAppContext();
  const { confirm, confirmDialog } = useConfirm();

  async function changeLanguage(value: Language) {
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("save"),
      description: t("confirmChangeLanguage"),
      title: t("confirmChangeLanguageTitle"),
    });
    if (confirmed) {
      setLanguage(value);
    }
  }

  async function changeTheme(value: ThemeMode) {
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("save"),
      description: t("confirmChangeTheme"),
      title: t("confirmChangeThemeTitle"),
    });
    if (confirmed) {
      setTheme(value);
    }
  }

  return (
    <Page>
      {confirmDialog}
      <PageHeader>
        <PageTitle visuallyHidden>{t("settings")}</PageTitle>
        <PageDescription>{t("externalAccess")}</PageDescription>
      </PageHeader>
      <PageBody className="settings-grid">
        <AccessPanel t={t} />
        <SectionCard title={t("language")}>
          <Field label={t("language")}>
            <Select
              value={language}
              onValueChange={(value) => void changeLanguage(value as Language)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh">中文</SelectItem>
                <SelectItem value="en">English</SelectItem>
              </SelectContent>
            </Select>
          </Field>
        </SectionCard>
        <SectionCard title={t("theme")}>
          <Field label={t("theme")}>
            <Select
              value={theme}
              onValueChange={(value) => void changeTheme(value as ThemeMode)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="system">{t("system")}</SelectItem>
                <SelectItem value="light">{t("light")}</SelectItem>
                <SelectItem value="dark">{t("dark")}</SelectItem>
              </SelectContent>
            </Select>
          </Field>
        </SectionCard>
      </PageBody>
    </Page>
  );
}

function AccessPanel({ t }: { t: (key: string) => string }) {
  const baseUrl = `${gatewayOrigin()}/v1`;
  const chatUrl = `${baseUrl}/chat/completions`;
  const modelsUrl = `${baseUrl}/models`;
  const curl = `curl ${chatUrl} -H "Authorization: Bearer <proxy-hub-api-key>" -H "Content-Type: application/json" -d "{\\"model\\":\\"<visible-model>\\",\\"messages\\":[{\\"role\\":\\"user\\",\\"content\\":\\"hi\\"}]}"`;

  return (
    <SectionCard className="settings-wide-card" title={t("externalAccess")}>
      <div className="endpoint-grid">
        <CopyValue
          copiedLabel={t("copied")}
          label={t("openAIBaseUrl")}
          value={baseUrl}
          t={t}
        />
        <CopyValue
          copiedLabel={t("copied")}
          label={t("chatCompletionsUrl")}
          value={chatUrl}
          t={t}
        />
        <CopyValue
          copiedLabel={t("copied")}
          label={t("modelsUrl")}
          value={modelsUrl}
          t={t}
        />
        <CopyValue
          copiedLabel={t("copied")}
          label={t("curlExample")}
          value={curl}
          t={t}
        />
      </div>
    </SectionCard>
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
