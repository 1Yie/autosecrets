import { useState } from "react";
import {
	Navigate,
	NavLink,
	Outlet,
	useLocation,
	useNavigate,
} from "react-router-dom";
import {
	ChevronRight,
	ChevronsUpDown,
	Info,
	KeyRound,
	LayoutDashboard,
	LogOut,
	MonitorCog,
	PanelLeftClose,
	PanelLeftOpen,
	Server,
	ScrollText,
	ShieldCheck,
} from "lucide-react";
import { useMe, type Me } from "../hooks/auth/use-me";
import { useLogout } from "../hooks/auth/use-logout";
import { useApplication } from "../hooks/applications/use-application";
import { AboutDialog } from "../components/about-dialog";
import { Button } from "../components/ui/button";
import { Skeleton } from "../components/ui/skeleton";
import { ThemeToggle } from "../components/theme-toggle";
import { SearchBox } from "../components/search-box";
import { ScrollArea } from "../components/ui/scroll-area";
import {
	Sheet,
	SheetHeader,
	SheetPanel,
	SheetPopup,
	SheetTitle,
	SheetTrigger,
} from "../components/ui/sheet";
import {
	Menu,
	MenuItem,
	MenuPopup,
	MenuSeparator,
	MenuTrigger,
} from "../components/ui/menu";
import { cn } from "../lib/utils";

const navItems = [
	{ to: "/dashboard/overview", label: "概览", icon: LayoutDashboard },
	{ to: "/dashboard/apps", label: "应用", icon: MonitorCog },
	{ to: "/dashboard/nodes", label: "节点", icon: Server },
	{ to: "/dashboard/audit", label: "审计", icon: ScrollText },
];

/** DashboardLayout: the authenticated console shell (collapsible sidebar,
 * top bar, and scrolling main). It gates on the session: unauthenticated or
 * incomplete-enrollment states redirect into the /auth/* surface. */
export function DashboardLayout() {
	const me = useMe();
	const [collapsed, setCollapsed] = useState(false);
	const [mobileOpen, setMobileOpen] = useState(false);

	if (me.isLoading) return <PageSkeleton />;
	if (me.data?.bootstrap_required)
		return <Navigate to="/auth/bootstrap" replace />;
	if (!me.data?.member) return <Navigate to="/auth/login" replace />;

	return (
		<div className="flex h-dvh overflow-hidden">
			<aside
				className={cn(
					"hidden shrink-0 flex-col border-r bg-sidebar transition-[width] duration-200 md:flex",
					collapsed ? "w-16" : "w-56",
				)}
			>
				<div
					className={cn(
						"flex h-14 shrink-0 items-center gap-2 border-b px-3",
						collapsed && "justify-center",
					)}
				>
					{collapsed ? (
						<Button
							type="button"
							size="icon-sm"
							variant="ghost"
							onClick={() => setCollapsed(false)}
							aria-label="展开侧边栏"
							className="opacity-60 hover:opacity-100"
						>
							<PanelLeftOpen />
						</Button>
					) : (
						<>
							<BrandMark />
							<Button
								type="button"
								size="icon-sm"
								variant="ghost"
								onClick={() => setCollapsed(true)}
								aria-label="收起侧边栏"
								className="ml-auto opacity-60 hover:opacity-100"
							>
								<PanelLeftClose />
							</Button>
						</>
					)}
				</div>
				<SidebarNav collapsed={collapsed} />
				<div className="shrink-0 border-t p-1.5">
					<UserMenu collapsed={collapsed} member={me.data.member} />
				</div>
			</aside>
			<div className="flex min-w-0 flex-1 flex-col">
				<header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
					<Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
						<SheetTrigger
							render={
								<Button
									type="button"
									size="icon-sm"
									variant="ghost"
									className="md:hidden"
								/>
							}
							aria-label="打开侧边栏"
						>
							<PanelLeftOpen />
						</SheetTrigger>
						<SheetPopup
							side="left"
							className="w-72 max-w-[calc(100vw-3rem)] bg-sidebar"
						>
							<SheetHeader className="border-b px-4 py-3">
								<SheetTitle className="text-sm font-semibold tracking-tight">
									<BrandMark />
								</SheetTitle>
							</SheetHeader>
							<SheetPanel className="p-0">
								<SidebarNav onNavigate={() => setMobileOpen(false)} />
							</SheetPanel>
							<div className="shrink-0 border-t p-1.5">
								<UserMenu collapsed={false} member={me.data.member} />
							</div>
						</SheetPopup>
					</Sheet>
					<div className="min-w-0 flex-1">
						<Breadcrumbs />
					</div>
					<div className="flex shrink-0 items-center gap-3">
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

function BrandMark() {
	return (
		<span className="flex items-center gap-2">
			<ShieldCheck className="size-5 shrink-0" />
			<span>AutoSecrets</span>
		</span>
	);
}

function SidebarNav({
	collapsed = false,
	onNavigate,
}: {
	collapsed?: boolean;
	onNavigate?: () => void;
}) {
	return (
		<nav className="flex-1 space-y-1 p-3">
			{navItems.map((item) => (
				<NavLink
					key={item.to}
					to={item.to}
					title={collapsed ? item.label : undefined}
					onClick={onNavigate}
					className={({ isActive }) =>
						cn(
							"flex items-center gap-2 overflow-hidden whitespace-nowrap rounded-md px-3 py-2 text-sm",
							collapsed && "justify-center px-0",
							isActive
								? "bg-sidebar-accent font-medium"
								: "opacity-70 hover:opacity-100",
						)
					}
				>
					<item.icon className="size-4 shrink-0" />
					{!collapsed && item.label}
				</NavLink>
			))}
		</nav>
	);
}

const sectionLabels: Record<string, string> = {
	overview: "概览",
	apps: "应用",
	nodes: "节点",
	audit: "审计",
	"login-and-security": "登录与安全",
};

/** Header breadcrumb derived from the current route, e.g. 应用 / payments. */
function Breadcrumbs() {
	const location = useLocation();
	const segments = location.pathname.split("/").filter(Boolean);
	if (segments[0] !== "dashboard" || segments.length < 2) return null;
	const section = segments[1];
	const label = sectionLabels[section];
	if (!label) return null;
	return (
		<nav aria-label="面包屑" className="flex min-w-0 items-center gap-1 text-sm">
			<span className="opacity-60">{label}</span>
			{section === "apps" && segments[2] && (
				<>
					<ChevronRight
						aria-hidden="true"
						className="size-3.5 shrink-0 opacity-40"
					/>
					<AppCrumb appId={segments[2]} />
				</>
			)}
		</nav>
	);
}

function AppCrumb({ appId }: { appId: string }) {
	const app = useApplication(appId);
	return (
		<span className="truncate font-medium">
			{app.data?.name ?? appId.slice(0, 8)}
		</span>
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
	const [aboutOpen, setAboutOpen] = useState(false);
	return (
		<>
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
								<span
									className="block truncate text-sm font-medium"
									data-testid="current-user"
								>
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
							<div className="truncate text-xs text-muted-foreground">
								{member.role}
							</div>
						</div>
					</div>
					<MenuSeparator />
					<MenuItem
						data-testid="account-security"
						onClick={() => navigate("/dashboard/login-and-security")}
					>
						<KeyRound className="size-4" />
						登录与安全
					</MenuItem>
					<MenuItem
						closeOnClick
						data-testid="about"
						onClick={() => setAboutOpen(true)}
					>
						<Info className="size-4" />
						关于 AutoSecrets
					</MenuItem>
					<MenuItem
						variant="destructive"
						data-testid="logout"
						onClick={() =>
							logout.mutate(undefined, {
								onSuccess: () => navigate("/auth/login"),
							})
						}
					>
						<LogOut className="size-4" />
						退出登录
					</MenuItem>
				</MenuPopup>
			</Menu>
			<AboutDialog open={aboutOpen} onOpenChange={setAboutOpen} />
		</>
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
