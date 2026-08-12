import { Outlet } from "react-router-dom";
import { useMe } from "../../hooks/auth/use-me";
import { BootstrapPage } from "../bootstrap";
import { LoginPage } from "../login";
import { ErrorBoundary } from "../../components/error-boundary";

/** Layout shell: decides bootstrap / login / authenticated shell, then the
 * routed page. All async states (loading/error/success) are explicit. */
export function AppLayout() {
  const me = useMe();

  if (me.isLoading) return <p className="p-6">Loading…</p>;

  if (me.isError && !me.data) {
    return (
      <Shell>
        <LoginPage />
      </Shell>
    );
  }
  if (me.data?.bootstrap_required) {
    return (
      <Shell>
        <BootstrapPage />
      </Shell>
    );
  }
  if (!me.data?.admin) {
    return (
      <Shell>
        <LoginPage />
      </Shell>
    );
  }
  return (
    <ErrorBoundary>
      <Shell>
        <Outlet />
      </Shell>
    </ErrorBoundary>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-4xl p-6">
      <header className="mb-6 flex items-center justify-between">
        <span className="text-lg font-bold tracking-tight">AutoSecrets</span>
        <nav className="flex gap-4 text-sm">
          <a href="/apps">Applications</a>
          <a href="/nodes">Nodes</a>
        </nav>
      </header>
      {children}
    </div>
  );
}
