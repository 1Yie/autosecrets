import { Navigate, Route, Routes } from "react-router-dom";
import { useMe } from "./hooks/useAuth";
import { AppPage } from "./pages/AppPage";
import { AppsPage } from "./pages/AppsPage";
import { BootstrapPage } from "./pages/BootstrapPage";
import { LoginPage } from "./pages/LoginPage";
import { NodesPage } from "./pages/NodesPage";

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-4xl p-6">
      <header className="mb-6 flex items-center justify-between">
        <span className="text-lg font-bold tracking-tight">AutoSecrets</span>
        <nav className="flex gap-4 text-sm">
          <a href="/apps">Applications</a>
          <a href="/nodes">Nodes</a>
          <a href="/apps">Audit</a>
        </nav>
      </header>
      {children}
    </div>
  );
}

export default function App() {
  const me = useMe();
  if (me.isLoading) return <p className="p-6">Loading…</p>;

  if (me.isError && !me.data) {
    // No session and no bootstrap state readable: treat as unauthenticated.
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
    <Shell>
      <Routes>
        <Route path="/" element={<Navigate to="/apps" replace />} />
        <Route path="/apps" element={<AppsPage />} />
        <Route path="/apps/:appId" element={<AppPage />} />
        <Route path="/nodes" element={<NodesPage />} />
        <Route path="*" element={<Navigate to="/apps" replace />} />
      </Routes>
    </Shell>
  );
}
