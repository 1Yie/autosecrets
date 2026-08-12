import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { LayoutDashboard, LogOut, MonitorCog, Server, ScrollText, ShieldCheck } from "lucide-react";
import { useMe } from "../hooks/auth/use-me";
import { useLogout } from "../hooks/auth/use-logout";
import { BootstrapPage } from "../pages/bootstrap";
import { LoginPage } from "../pages/login";
import { ErrorBoundary } from "../components/error-boundary";
import { Skeleton } from "../components/ui/skeleton";
import { ThemeToggle } from "../components/theme-toggle";
import { SearchBox } from "../components/search-box";
import { cn } from "../lib/utils";

const navItems = [
  { to: "/overview", label: "概览", icon: LayoutDashboard },
  { to: "/apps", label: "应用", icon: MonitorCog },
  { to: "/nodes", label: "节点", icon: Server },
  { to: "/audit", label: "审计", icon: ScrollText },
];

/** Layout shell: dedicated AuthLayout for bootstrap/login, then the
 * authenticated console Shell with collapsible navigation and top bar. */
export function AppLayout() {
  const me = useMe();

  if (me.isLoading) return <PageSkeleton />;

  if (me.data?.bootstrap_required) {
    return (
      <AuthLayout>
        <BootstrapPage />
      </AuthLayout>
    );
  }
  if (me.data?.mfa_enrollment_required) {
    return (
      <AuthLayout>
        <div className="space-y-4">
          <h1 className="text-xl font-bold">首次管理员注册未完成</h1>
          <p className="text-sm opacity-70">
            系统已存在待激活的首位管理员。请刷新页面重试；若仍无法继续，请通过 Core
            主机本地流程处理。
          </p>
          <button
            className="text-sm text-blue-600 underline"
            onClick={() => window.location.reload()}
          >
            刷新
          </button>
        </div>
      </AuthLayout>
    );
  }
  if (!me.data?.member) {
    return (
      <AuthLayout>
        <LoginPage />
      </AuthLayout>
    );
  }
  return (
    <ErrorBoundary>
      <div className="flex min-h-dvh">
        <aside className="hidden w-56 shrink-0 flex-col border-r bg-sidebar md:flex">
          <div className="flex h-14 items-center gap-2 border-b px-4">
            <ShieldCheck className="size-5" />
            <span className="text-sm font-semibold tracking-tight">AutoSecrets</span>
          </div>
          <nav className="flex-1 space-y-1 p-3">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  cn(
                    "flex items-center gap-2 rounded-md px-3 py-2 text-sm",
                    isActive ? "bg-sidebar-accent font-medium" : "opacity-70 hover:opacity-100",
                  )
                }
              >
                <item.icon className="size-4" />
                {item.label}
              </NavLink>
            ))}
          </nav>
        </aside>
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex h-14 items-center justify-between border-b px-4">
            <div className="text-sm opacity-70">
              {me.data.organization?.display_name ?? "AutoSecrets"}
            </div>
            <div className="flex items-center gap-3">
              <SearchBox />
              <ThemeToggle />
              <span className="text-sm" data-testid="current-user">
                {me.data.member.username}
              </span>
              <LogoutButton />
            </div>
          </header>
          <main className="min-w-0 flex-1 overflow-auto p-4 md:p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </ErrorBoundary>
  );
}

function LogoutButton() {
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <button
      className="flex items-center gap-1 text-sm opacity-70 hover:opacity-100"
      onClick={() => logout.mutate(undefined, { onSuccess: () => navigate("/login") })}
      data-testid="logout"
    >
      <LogOut className="size-4" />
      退出
    </button>
  );
}

/** AuthLayout: dedicated left-right authentication context shared by
 * Bootstrap and login. The left panel states the product purpose without
 * exposing live data; the right panel carries the form. */
function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-dvh md:grid-cols-[1fr_1.1fr]">
      <div className="hidden flex-col justify-between bg-sidebar p-10 md:flex">
        <div className="flex items-center gap-2">
          <ShieldCheck className="size-5" />
          <span className="font-semibold tracking-tight">AutoSecrets</span>
        </div>
        <div className="space-y-3 text-sm opacity-80">
          <p className="text-base font-medium">Secret 的受控下发</p>
          <p className="opacity-70">
            Application → Environment → Bundle Revision → Node Group → Managed Node
          </p>
          <p className="opacity-50">每次 Publish 都需要 Operation Reason；受保护环境需要最近一次密码确认。</p>
        </div>
        <div className="text-xs opacity-50">Self-hosted Secret 控制面 · 管理面与 Agent 面分离</div>
      </div>
      <div className="flex items-center justify-center p-6">
        <div className="w-full max-w-md space-y-6">{children}</div>
      </div>
    </div>
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
