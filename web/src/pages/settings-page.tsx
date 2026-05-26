import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Field } from "../components/ui/field";
import { Select } from "../components/ui/select";
import { useAppContext } from "../lib/app-context";
import type { Language, ThemeMode } from "../lib/types";

export function SettingsPage() {
  const { language, setLanguage, setTheme, t, theme } = useAppContext();

  return (
    <section className="settings-grid">
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
