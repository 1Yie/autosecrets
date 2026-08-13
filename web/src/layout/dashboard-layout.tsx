import { useState } from "react";
import { Navigate, NavLink, Outlet, useNavigate } from "react-router-dom";
import { ChevronsUpDown, KeyRound, LayoutDashboard, LogOut, MonitorCog, PanelLeftClose, PanelLeftOpen, Server, ScrollText, ShieldCheck } from "lucide-react";
import { useMe, type Me } from "../hooks/auth/use-me";
import { useLogout } from "../hooks/auth/use-logout";
import { Skeleton } from "../components/ui/skeleton";
import { ThemeToggle } from "../components/theme-toggle";
import { SearchBox } from "../components/search-box";
import { ScrollArea } from "../components/ui/scroll-area";
import { Menu, MenuItem, MenuPopup, MenuSeparator, MenuTrigger } from "../components/ui/menu";
import { cn } from "../lib/utils";

const navItems = [
  { to: "/dashboard/overview", label: "概览", icon: LayoutDashboard },
  { to: "/dashboard/apps", label: "应用", icon: MonitorCog },
  { to: "/dashboard/nodes", label: "节点", icon: Server },
  { to: "/dashboard/audit", label: "审计", icon: ScrollText },
  { to: "/dashboard/security", label: "登录安全", icon: KeyRound },
];

/** DashboardLayout: the authenticated console shell (collapsible sidebar,
 * top bar, and scrolling main). It gates on the session: unauthenticated or
 * incomplete-enrollment states redirect into the /auth/* surface. */
export function DashboardLayout() {
  const me = useMe();
  const [collapsed, setCollapsed] = useState(false);

  if (me.isLoading) return <PageSkeleton />;
  if (me.data?.bootstrap_required) return <Navigate to="/auth/bootstrap" replace />;
  if (!me.data?.member) return <Navigate to="/auth/login" replace />;

  return (
    <div className="flex h-dvh overflow-hidden">
      <aside className={cn(
        "hidden shrink-0 flex-col border-r bg-sidebar transition-[width] duration-200 md:flex",
        collapsed ? "w-16" : "w-56",
      )}>
        <div className={cn(
          "flex h-14 shrink-0 items-center gap-2 border-b px-3",
          collapsed && "justify-center",
        )}>
          {collapsed ? (
            <button
              type="button"
              onClick={() => setCollapsed(false)}
              aria-label="展开侧边栏"
              className="flex size-8 items-center justify-center rounded-md opacity-60 hover:bg-sidebar-accent hover:opacity-100"
            >
              <PanelLeftOpen className="size-4" />
            </button>
          ) : (
            <>
              <ShieldCheck className="size-5 shrink-0" />
              <span className="text-sm font-semibold tracking-tight">AutoSecrets</span>
              <button
                type="button"
                onClick={() => setCollapsed(true)}
                aria-label="收起侧边栏"
                className="ml-auto flex size-8 shrink-0 items-center justify-center rounded-md opacity-60 hover:bg-sidebar-accent hover:opacity-100"
              >
                <PanelLeftClose className="size-4" />
              </button>
            </>
          )}
        </div>
        <nav className="flex-1 space-y-1 p-3">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              title={collapsed ? item.label : undefined}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm",
                  collapsed && "justify-center px-0",
                  isActive ? "bg-sidebar-accent font-medium" : "opacity-70 hover:opacity-100",
                )
              }
            >
              <item.icon className="size-4 shrink-0" />
              {!collapsed && item.label}
            </NavLink>
          ))}
        </nav>
        <div className="shrink-0 border-t p-1.5">
          <UserMenu collapsed={collapsed} member={me.data.member} />
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between border-b px-4">
          <div className="text-sm opacity-70">
            {me.data.organization?.display_name ?? "AutoSecrets"}
          </div>
          <div className="flex items-center gap-3">
            <SearchBox />
            <ThemeToggle />
          </div>
        </header>
        <main className="min-h-0 min-w-0 flex-1">
          <ScrollArea className="size-full">
            <div className="p-4 md:p-6">
              <div className="mx-auto max-w-7xl">
                <Outlet />
              </div>
            </div>
          </ScrollArea>
        </main>
      </div>
    </div>
  );
}

function UserMenu({
  collapsed,
  member,
}: {
  collapsed: boolean;
  member: NonNullable<Me["member"]>;
}) {
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <Menu>
      <MenuTrigger
        className={cn(
          "flex w-full items-center gap-2 rounded-md text-left opacity-90 hover:bg-sidebar-accent hover:opacity-100",
          collapsed ? "justify-center px-0 py-2" : "px-2 py-1.5",
        )}
      >
        <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-sm font-medium">
          {member.username.slice(0, 1).toUpperCase()}
        </span>
        {!collapsed && (
          <>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium" data-testid="current-user">
                {member.username}
              </span>
              <span className="block truncate text-xs opacity-60">{member.role}</span>
            </span>
            <ChevronsUpDown className="size-3.5 shrink-0 opacity-50" />
          </>
        )}
      </MenuTrigger>
      <MenuPopup side="right" align="start" sideOffset={8}>
        <div className="flex items-center gap-2 px-2 py-1.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-sm font-medium">
            {member.username.slice(0, 1).toUpperCase()}
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{member.username}</div>
            <div className="truncate text-xs text-muted-foreground">{member.role}</div>
          </div>
        </div>
        <MenuSeparator />
        <MenuItem
          variant="destructive"
          data-testid="logout"
          onClick={() => logout.mutate(undefined, { onSuccess: () => navigate("/auth/login") })}
        >
          <LogOut className="size-4" />
          退出登录
        </MenuItem>
      </MenuPopup>
    </Menu>
  );
}

function PageSkeleton() {
  return (
    <div className="mx-auto max-w-4xl space-y-4 p-6">
      <Skeleton className="h-6 w-40" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  );
}

export default DashboardLayout;
