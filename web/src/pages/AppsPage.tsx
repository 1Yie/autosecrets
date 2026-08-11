import { useState } from "react";
import { Link } from "react-router-dom";
import { useApplications, useCreateApplication } from "../hooks/useApps";

export function AppsPage() {
  const apps = useApplications();
  const create = useCreateApplication();
  const [name, setName] = useState("");

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">Applications</h1>
      {apps.isLoading && <p>Loading…</p>}
      {apps.isError && <p className="text-red-500">Failed to load applications</p>}
      {apps.data?.length === 0 && (
        <p className="opacity-60">No applications yet. Create one to begin.</p>
      )}
      <ul className="space-y-2">
        {apps.data?.map((app) => (
          <li key={app.id}>
            <Link to={`/apps/${app.id}`} className="block rounded border p-3 hover:bg-white/5">
              {app.name}
            </Link>
          </li>
        ))}
      </ul>
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) {
            create.mutate(name.trim());
            setName("");
          }
        }}
      >
        <input
          className="flex-1 rounded border p-2"
          placeholder="Application name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          data-testid="app-name"
        />
        <button className="rounded bg-amber-500 px-4 font-semibold" type="submit">
          Create
        </button>
      </form>
    </div>
  );
}
