import { Outlet } from "react-router-dom";
import { ShieldCheck } from "lucide-react";

/** AuthLayout: the left-right authentication shell for the /auth/* routes
 * (login, bootstrap, and resumed MFA enrollment). The left panel states the
 * product purpose without exposing live data; the right panel carries the
 * form via <Outlet />. */
export function AuthLayout() {
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
          <p className="opacity-50">Application、Environment 和 Node Group 分开管理，发布后节点自动同步。</p>
        </div>
        <div className="text-xs opacity-50">Self-hosted Secret 控制面 · 管理面与 Agent 面分离</div>
      </div>
      <div className="flex items-center justify-center p-6">
        <div className="w-full max-w-md space-y-6">
          <Outlet />
        </div>
      </div>
    </div>
  );
}

export default AuthLayout;
