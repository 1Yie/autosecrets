import { Link } from "react-router-dom";
import { Button } from "../../components/ui/button";

/** NotFoundPage: rendered for unknown /dashboard/* and unknown top-level
 * routes instead of silently bouncing the user to the overview. */
export function NotFoundPage() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
      <p className="text-6xl font-bold tracking-tight opacity-20">404</p>
      <h1 className="text-xl font-bold">页面不存在</h1>
      <p className="max-w-sm text-sm opacity-60">
        你访问的页面不存在，或已被移动。请从侧边栏选择页面，或返回概览。
      </p>
      <Button variant="default" render={<Link to="/dashboard/overview" />}>
        返回概览
      </Button>
    </div>
  );
}

export default NotFoundPage;
