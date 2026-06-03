import {
  Activity,
  ChevronLeft,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import logoUrl from "../../assets/logo.png";
import { api } from "../../lib/api";
import { useAppContext } from "../../lib/app-context";
import { consoleRoutes, titleForPath } from "../../lib/navigation";
import type { Language, ThemeMode } from "../../lib/types";
import { cn } from "../../lib/cn";
import { Button } from "../ui/button";
import { Card } from "../ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/select";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "../ui/sheet";
import { useConfirm } from "../ui/use-confirm";

export function AppShell() {
  const { language, setLanguage, setLoggedOut, setTheme, t, theme, username } = useAppContext();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
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
    <div className={cn("app-shell", sidebarCollapsed && "app-shell-sidebar-collapsed")}>
      <DesktopSidebar collapsed={sidebarCollapsed} routeTitle={routeTitle} username={username} />
      <div className="app-shell-main">
        <header className="app-topbar">
          <div className="app-topbar-title">
            <div className="app-topbar-leading">
              <Sheet onOpenChange={setMobileNavOpen} open={mobileNavOpen}>
                <SheetTrigger asChild>
                  <Button aria-label={t("expandSidebar")} className="app-mobile-nav-trigger" size="icon" type="button" variant="outline">
                    <Menu size={18} />
                  </Button>
                </SheetTrigger>
                <SheetContent className="app-mobile-nav-sheet">
                  <SheetHeader className="app-mobile-nav-header">
                    <SheetTitle>{t("appName")}</SheetTitle>
                  </SheetHeader>
                  <MobileSidebarContent onNavigate={() => setMobileNavOpen(false)} routeTitle={routeTitle} username={username} />
                </SheetContent>
              </Sheet>
              <Button
                aria-label={sidebarCollapsed ? t("expandSidebar") : t("collapseSidebar")}
                className="app-sidebar-toggle"
                onClick={() => setSidebarCollapsed((value) => !value)}
                size="icon"
                type="button"
                variant="outline"
              >
                {sidebarCollapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
              </Button>
              {CurrentIcon && (
                <span aria-hidden="true" className="app-topbar-icon">
                  <CurrentIcon size={18} />
                </span>
              )}
            </div>
            <div className="app-topbar-copy">
              <h1>{routeTitle}</h1>
              <span className="app-topbar-subtitle">{username ? `${t("appName")} · ${username}` : t("appName")}</span>
            </div>
          </div>
          <div className="app-topbar-actions">
            <Card className="app-status-pill">
              <div className="app-status-pill-content">
                <Activity size={16} />
                <span>{t("gatewayReady")}</span>
              </div>
            </Card>
            <div aria-label="Display preferences" className="app-preference-group">
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
            <Button onClick={() => void logout()} type="button" variant="outline">
              <LogOut size={16} />
              {t("logout")}
            </Button>
          </div>
        </header>
        {confirmDialog}
        <main className="app-shell-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function DesktopSidebar({
  collapsed,
  routeTitle,
  username,
}: {
  collapsed: boolean;
  routeTitle: string;
  username: string;
}) {
  return (
    <aside className="app-sidebar">
      <SidebarContent collapsed={collapsed} routeTitle={routeTitle} username={username} />
    </aside>
  );
}

function MobileSidebarContent({
  onNavigate,
  routeTitle,
  username,
}: {
  onNavigate: () => void;
  routeTitle: string;
  username: string;
}) {
  return <SidebarContent onNavigate={onNavigate} routeTitle={routeTitle} username={username} />;
}

function SidebarContent({
  collapsed = false,
  onNavigate,
  routeTitle,
  username,
}: {
  collapsed?: boolean;
  onNavigate?: () => void;
  routeTitle: string;
  username: string;
}) {
  const { t } = useAppContext();

  return (
    <div className={cn("app-sidebar-panel", collapsed && "app-sidebar-panel-collapsed")}>
      <div className="app-sidebar-brand">
        <div className="app-sidebar-brand-mark">
          <img alt="" aria-hidden="true" className="app-sidebar-brand-logo" src={logoUrl} />
        </div>
        <div className="app-sidebar-brand-copy">
          <strong>{t("appName")}</strong>
          <span>{username}</span>
        </div>
      </div>

      <nav aria-label="Main navigation" className="app-sidebar-nav">
        {consoleRoutes.map((item) => {
          const Icon = item.icon;
          const link = (
            <NavLink className={({ isActive }) => cn("app-sidebar-link", isActive && "is-active")} key={item.path} to={item.path}>
              <span className="app-sidebar-link-icon">
                <Icon size={18} />
              </span>
              <span className="app-sidebar-link-label">{t(item.labelKey)}</span>
            </NavLink>
          );

          if (!onNavigate) {
            return link;
          }

          return (
            <SheetClose asChild key={item.path}>
              {link}
            </SheetClose>
          );
        })}
      </nav>

      <div className="app-sidebar-footer">
        <span>{routeTitle}</span>
        <ChevronLeft size={14} />
      </div>
    </div>
  );
}
