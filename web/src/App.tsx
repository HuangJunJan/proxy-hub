import { useEffect, useMemo, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { ScreenCenter } from "./components/screen-center";
import { AppShell } from "./components/layout/app-shell";
import { api } from "./lib/api";
import { AppContextProvider } from "./lib/app-context";
import { translate } from "./lib/i18n";
import { applyTheme } from "./lib/theme";
import type { Language, ThemeMode } from "./lib/types";
import { usePersistentState } from "./lib/use-persistent-state";
import { ChannelsPage } from "./pages/channels-page";
import { DashboardPage } from "./pages/dashboard-page";
import { KeysPage } from "./pages/keys-page";
import { LivePage } from "./pages/live-page";
import { LoginPage } from "./pages/login-page";
import { LogsPage } from "./pages/logs-page";
import { SettingsPage } from "./pages/settings-page";
import { SetupPage } from "./pages/setup-page";

type BootMode = "checking" | "setup" | "login" | "app";

export function App() {
  const [language, setLanguage] = usePersistentState<Language>("proxy-hub-language", "zh");
  const [theme, setTheme] = usePersistentState<ThemeMode>("proxy-hub-theme", "system");
  const [mode, setMode] = useState<BootMode>("checking");
  const [username, setUsername] = useState("");

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    let cancelled = false;
    async function boot() {
      try {
        const status = await api.setupStatus();
        if (cancelled) {
          return;
        }
        if (status.needed) {
          setMode("setup");
          return;
        }
        const me = await api.me();
        if (cancelled) {
          return;
        }
        setUsername(me.username);
        setMode("app");
      } catch {
        if (!cancelled) {
          setMode("login");
        }
      }
    }
    void boot();
    return () => {
      cancelled = true;
    };
  }, []);

  const contextValue = useMemo(
    () => ({
      language,
      setAuthenticated: (nextUsername: string) => {
        setUsername(nextUsername);
        setMode("app");
      },
      setLanguage,
      setLoggedOut: () => {
        setUsername("");
        setMode("login");
      },
      setTheme,
      t: (key: string) => translate(language, key),
      theme,
      username,
    }),
    [language, setLanguage, setTheme, theme, username],
  );

  if (mode === "checking") {
    return (
      <ScreenCenter>
        <span className="spinner" />
        {contextValue.t("loading")}
      </ScreenCenter>
    );
  }

  return (
    <AppContextProvider value={contextValue}>
      <Routes>
        {mode === "setup" && (
          <>
            <Route element={<SetupPage />} path="/setup" />
            <Route element={<Navigate replace to="/setup" />} path="*" />
          </>
        )}
        {mode === "login" && (
          <>
            <Route element={<LoginPage />} path="/login" />
            <Route element={<Navigate replace to="/login" />} path="*" />
          </>
        )}
        {mode === "app" && (
          <Route element={<AppShell />} path="/">
            <Route element={<Navigate replace to="/dashboard" />} index />
            <Route element={<DashboardPage />} path="dashboard" />
            <Route element={<ChannelsPage />} path="channels" />
            <Route element={<KeysPage />} path="keys" />
            <Route element={<LogsPage />} path="logs" />
            <Route element={<LivePage />} path="live" />
            <Route element={<SettingsPage />} path="settings" />
            <Route element={<Navigate replace to="/dashboard" />} path="*" />
          </Route>
        )}
      </Routes>
    </AppContextProvider>
  );
}
