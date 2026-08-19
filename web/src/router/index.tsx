/* eslint-disable react-refresh/only-export-components -- route table file */
import { lazy, Suspense } from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { AuthLayout } from "../layout/auth-layout";
import { DashboardLayout } from "../layout/dashboard-layout";
import { ErrorBoundary } from "../components/error-boundary";
import { Spinner } from "../components/ui/spinner";
import { NotFoundPage } from "../pages/not-found";

const AppsPage = lazy(() => import("../pages/apps"));
const AppPage = lazy(() => import("../pages/app"));
const NodesPage = lazy(() => import("../pages/nodes"));
const OverviewPage = lazy(() => import("../pages/overview"));
const AuditPage = lazy(() => import("../pages/audit"));
const SecurityPage = lazy(() => import("../pages/security"));
const LoginPage = lazy(() => import("../pages/login"));
const BootstrapPage = lazy(() => import("../pages/bootstrap"));

function lazyPage(element: React.ReactNode) {
	return (
		<Suspense
			fallback={
				<div className="flex justify-center p-12">
					<Spinner />
				</div>
			}
		>
			{element}
		</Suspense>
	);
}

/** Centralized route table: /auth/* for unauthenticated flows, /dashboard/*
 * for the authenticated console. Route-level Error Boundary via errorElement. */
export const router = createBrowserRouter([
	{
		path: "/",
		element: <Navigate to="/dashboard/overview" replace />,
	},
	{
		path: "/auth",
		element: <AuthLayout />,
		errorElement: <ErrorBoundary />,
		children: [
			{ index: true, element: <Navigate to="/auth/login" replace /> },
			{ path: "login", element: lazyPage(<LoginPage />) },
			{ path: "bootstrap", element: lazyPage(<BootstrapPage />) },
		],
	},
	{
		path: "/dashboard",
		element: <DashboardLayout />,
		errorElement: <ErrorBoundary />,
		children: [
			{ index: true, element: <Navigate to="/dashboard/overview" replace /> },
			{ path: "overview", element: lazyPage(<OverviewPage />) },
			{ path: "apps", element: lazyPage(<AppsPage />) },
			{ path: "apps/:appId", element: lazyPage(<AppPage />) },
			{ path: "nodes", element: lazyPage(<NodesPage />) },
			{ path: "audit", element: lazyPage(<AuditPage />) },
			{ path: "settings", element: lazyPage(<SecurityPage />) },
			{
				path: "login-and-security",
				element: <Navigate to="/dashboard/settings" replace />,
			},
			{ path: "*", element: <NotFoundPage /> },
		],
	},
	{ path: "*", element: <NotFoundPage /> },
]);
