import { LogOut } from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { useAppContext } from "../../lib/app-context";
import { consoleRoutes, titleForPath } from "../../lib/navigation";
import type { Language, ThemeMode } from "../../lib/types";
import { Button } from "../ui/button";
import { Select } from "../ui/select";

export function AppShell() {
  const { language, setLanguage, setLoggedOut, setTheme, t, theme, username } = useAppContext();
  const location = useLocation();
  const navigate = useNavigate();

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">P</div>
          <div>
            <strong>{t("appName")}</strong>
            <span>{username}</span>
          </div>
        </div>
        <nav>
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
          <h1>{t(titleForPath(location.pathname))}</h1>
          <div className="topbar-actions">
            <Select value={language} onChange={(event) => setLanguage(event.target.value as Language)}>
              <option value="zh">中文</option>
              <option value="en">English</option>
            </Select>
            <Select value={theme} onChange={(event) => setTheme(event.target.value as ThemeMode)}>
              <option value="system">{t("system")}</option>
              <option value="light">{t("light")}</option>
              <option value="dark">{t("dark")}</option>
            </Select>
            <Button
              onClick={async () => {
                await api.logout();
                setLoggedOut();
                navigate("/login");
              }}
              type="button"
              variant="outline"
            >
              <LogOut size={16} />
              {t("logout")}
            </Button>
          </div>
        </header>
        <Outlet />
      </main>
    </div>
  );
}
