/* eslint-disable react-refresh/only-export-components -- route table file */
import { lazy, Suspense } from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { AppLayout } from "../pages/layout";
import { ErrorBoundary } from "../components/error-boundary";

const AppsPage = lazy(() => import("../pages/apps"));
const AppPage = lazy(() => import("../pages/app"));
const NodesPage = lazy(() => import("../pages/nodes"));

function lazyPage(element: React.ReactNode) {
  return (
    <Suspense fallback={<p className="p-6">Loading…</p>}>{element}</Suspense>
  );
}

/** Centralized route table (frontend-guidelines: src/router/index.ts with a
 * data router; route-level Error Boundary via errorElement). */
export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppLayout />,
    errorElement: <ErrorBoundary />,
    children: [
      { index: true, element: <Navigate to="/apps" replace /> },
      { path: "apps", element: lazyPage(<AppsPage />) },
      { path: "apps/:appId", element: lazyPage(<AppPage />) },
      { path: "nodes", element: lazyPage(<NodesPage />) },
      { path: "*", element: <Navigate to="/apps" replace /> },
    ],
  },
]);
