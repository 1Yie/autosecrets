/** Top navigation shared by every authenticated screen. */
export function Header() {
  return (
    <header className="mb-6 flex items-center justify-between">
      <span className="text-lg font-bold tracking-tight">AutoSecrets</span>
      <nav className="flex gap-4 text-sm">
        <a href="/apps">Applications</a>
        <a href="/nodes">Nodes</a>
      </nav>
    </header>
  );
}
