import { Link } from "react-router-dom";

/** Top navigation shared by every authenticated screen. */
export function Header() {
  return (
    <header className="mb-6 flex items-center justify-between">
      <span className="text-lg font-bold tracking-tight">AutoSecrets</span>
      <nav className="flex gap-4 text-sm">
        <Link to="/apps">应用</Link>
        <Link to="/nodes">节点</Link>
      </nav>
    </header>
  );
}
