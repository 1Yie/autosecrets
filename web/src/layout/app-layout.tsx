import { Outlet } from "react-router-dom";
import { useMe } from "../hooks/auth/use-me";
import { BootstrapPage } from "../pages/bootstrap";
import { LoginPage } from "../pages/login";
import { ErrorBoundary } from "../components/error-boundary";
import { Skeleton } from "../components/ui/skeleton";
import { Header } from "../components/header";

/** Layout shell: decides bootstrap / login / authenticated shell, then the
 * routed page. All async states (loading/error/success) are explicit. */
export function AppLayout() {
  const me = useMe();

  if (me.isLoading) return <PageSkeleton />;

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
      <Header />
      {children}
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
