import { useState } from "react";
import { useLogin } from "../hooks/useAuth";

export function LoginPage() {
  const login = useLogin();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  return (
    <div className="mx-auto mt-16 max-w-md space-y-4 rounded-lg border p-6">
      <h1 className="text-xl font-bold">Sign in</h1>
      <input className="w-full rounded border p-2" placeholder="Username"
        value={username} onChange={(e) => setUsername(e.target.value)} data-testid="username" />
      <input className="w-full rounded border p-2" type="password" placeholder="Password"
        value={password} onChange={(e) => setPassword(e.target.value)} data-testid="password" />
      {login.isError && (
        <p className="text-sm text-red-500">{String((login.error as Error).message)}</p>
      )}
      <button
        className="w-full rounded bg-amber-500 p-2 font-semibold disabled:opacity-50"
        disabled={login.isPending}
        onClick={() => login.mutate({ username, password })}
      >
        Sign in
      </button>
    </div>
  );
}
