import { useState } from "react";
import { useBootstrap } from "../hooks/useAuth";

export function BootstrapPage() {
  const bootstrap = useBootstrap();
  const [code, setCode] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  return (
    <div className="mx-auto mt-16 max-w-md space-y-4 rounded-lg border p-6">
      <h1 className="text-xl font-bold">Initialize AutoSecrets</h1>
      <p className="text-sm opacity-70">
        Paste the one-time bootstrap code from the Core logs
        (<code>docker compose logs core</code>) to create the first
        Administrator. There is no default credential.
      </p>
      <input className="w-full rounded border p-2" placeholder="Bootstrap code"
        value={code} onChange={(e) => setCode(e.target.value)} data-testid="code" />
      <input className="w-full rounded border p-2" placeholder="Username"
        value={username} onChange={(e) => setUsername(e.target.value)} data-testid="username" />
      <input className="w-full rounded border p-2" type="password" placeholder="Password (min 10 chars)"
        value={password} onChange={(e) => setPassword(e.target.value)} data-testid="password" />
      {bootstrap.isError && (
        <p className="text-sm text-red-500">
          {String((bootstrap.error as Error).message)}
        </p>
      )}
      <button
        className="w-full rounded bg-amber-500 p-2 font-semibold disabled:opacity-50"
        disabled={!code || !username || password.length < 10 || bootstrap.isPending}
        onClick={() => bootstrap.mutate({ code, username, password })}
      >
        Create administrator
      </button>
    </div>
  );
}
