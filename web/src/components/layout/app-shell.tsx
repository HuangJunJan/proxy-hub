import { Activity, LogOut } from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { useAppContext } from "../../lib/app-context";
import { consoleRoutes, titleForPath } from "../../lib/navigation";
import type { Language, ThemeMode } from "../../lib/types";
import { useConfirm } from "../ui/use-confirm";
import { Button } from "../ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/select";
import logoUrl from "../../assets/logo.png";

export function AppShell() {
  const { language, setLanguage, setLoggedOut, setTheme, t, theme, username } = useAppContext();
  const { confirm, confirmDialog } = useConfirm();
  const location = useLocation();
  const navigate = useNavigate();
  const currentRoute = consoleRoutes.find((item) => item.path === location.pathname);
  const CurrentIcon = currentRoute?.icon;
  const routeTitle = t(titleForPath(location.pathname));

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

  async function logout() {
    const confirmed = await confirm({
      cancelLabel: t("cancel"),
      confirmLabel: t("logout"),
      description: t("confirmLogout"),
      title: t("confirmLogoutTitle"),
      tone: "destructive",
    });
    if (!confirmed) {
      return;
    }
    await api.logout();
    setLoggedOut();
    navigate("/login");
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <img alt="" aria-hidden="true" className="brand-logo" src={logoUrl} />
          </div>
          <div>
            <strong>{t("appName")}</strong>
            <span>{username}</span>
          </div>
        </div>
        <nav aria-label="Main navigation" className="sidebar-nav">
          {consoleRoutes.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink className={({ isActive }) => (isActive ? "nav-item active" : "nav-item")} key={item.path} to={item.path}>
                <span className="nav-icon">
                  <Icon size={18} />
                </span>
                {t(item.labelKey)}
              </NavLink>
            );
          })}
        </nav>
      </aside>
      <main className="workspace">
        <header className="topbar">
          <div className="topbar-title">
            {CurrentIcon && (
              <span className="topbar-icon" aria-hidden="true">
                <CurrentIcon size={18} />
              </span>
            )}
            <div>
              <h1>{routeTitle}</h1>
              <span className="topbar-subtitle">{username ? `${t("appName")} · ${username}` : t("appName")}</span>
            </div>
          </div>
          <div className="topbar-actions">
            <div className="gateway-status">
              <Activity size={16} />
              <span>{t("gatewayReady")}</span>
            </div>
            <div className="topbar-control-group" aria-label="Display preferences">
              <Select value={language} onValueChange={(value) => void changeLanguage(value as Language)}>
                <SelectTrigger aria-label="Language">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="zh">中文</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                </SelectContent>
              </Select>
              <Select value={theme} onValueChange={(value) => void changeTheme(value as ThemeMode)}>
                <SelectTrigger aria-label="Theme">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="system">{t("system")}</SelectItem>
                  <SelectItem value="light">{t("light")}</SelectItem>
                  <SelectItem value="dark">{t("dark")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button
              onClick={() => void logout()}
              type="button"
              variant="outline"
            >
              <LogOut size={16} />
              {t("logout")}
            </Button>
          </div>
        </header>
        {confirmDialog}
        <Outlet />
      </main>
    </div>
  );
}
