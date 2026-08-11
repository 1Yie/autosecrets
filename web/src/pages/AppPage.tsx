import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  useApplication,
  useCreateEnvironment,
  useCreateSecret,
  useCreateVersion,
  useDraft,
  usePublish,
  useRevisions,
  useSecrets,
  useUpdateBinding,
  type SecretRow,
} from "../hooks/useApps";

function RotateButton({ secret, appId, envId }: { secret: SecretRow; appId: string; envId: string }) {
  const rotate = useCreateVersion(secret.id, appId, envId);
  const [value, setValue] = useState("");
  return (
    <span className="flex gap-1">
      <input
        className="rounded border p-1"
        placeholder="new value"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        data-testid={`rotate-${secret.name}`}
      />
      <button
        className="rounded border px-2 py-1 disabled:opacity-40"
        disabled={!value || rotate.isPending}
        onClick={() => {
          rotate.mutate(value);
          setValue("");
        }}
      >
        Rotate
      </button>
    </span>
  );
}

function BindingRow({ secret, appId, envId }: { secret: SecretRow; appId: string; envId: string }) {
  const update = useUpdateBinding(secret.id, appId, envId);
  const [path, setPath] = useState(secret.binding?.path ?? "");
  const [mode, setMode] = useState(secret.binding?.mode ?? "0400");
  return (
    <span className="flex gap-1">
      <input
        className="flex-1 rounded border p-1 font-mono"
        value={path}
        onChange={(e) => setPath(e.target.value)}
        data-testid={`binding-${secret.name}`}
      />
      <select className="rounded border p-1" value={mode} onChange={(e) => setMode(e.target.value)}>
        {["0400", "0440", "0444", "0600", "0640", "0644"].map((m) => (
          <option key={m} value={m}>{m}</option>
        ))}
      </select>
      <button
        className="rounded border px-2 py-1 disabled:opacity-40"
        disabled={!path.trim() || update.isPending}
        onClick={() =>
          update.mutate({ path: path.trim(), uid: 0, gid: 0, mode })
        }
      >
        Save
      </button>
    </span>
  );
}

function SecretEditor({ appId, envId }: { appId: string; envId: string }) {
  const secrets = useSecrets(appId, envId);
  const create = useCreateSecret(appId, envId);
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);
  const revisions = useRevisions(appId, envId);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");

  return (
    <div className="space-y-6">
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim() && value) {
            create.mutate({ name: name.trim(), value });
            setName("");
            setValue("");
          }
        }}
      >
        <input className="flex-1 rounded border p-2" placeholder="Secret name"
          value={name} onChange={(e) => setName(e.target.value)} data-testid="secret-name" />
        <input className="flex-1 rounded border p-2" placeholder="Secret value"
          value={value} onChange={(e) => setValue(e.target.value)} data-testid="secret-value" />
        <button className="rounded bg-amber-500 px-4 font-semibold" type="submit">
          Add secret
        </button>
      </form>

      {secrets.isLoading && <p>Loading secrets…</p>}
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b opacity-60">
            <th className="p-2">Name</th>
            <th className="p-2">Binding</th>
            <th className="p-2">Version</th>
            <th className="p-2">Rotate</th>
          </tr>
        </thead>
        <tbody>
          {secrets.data?.map((s) => (
            <tr key={s.id} className="border-b">
              <td className="p-2 font-mono">{s.name}</td>
              <td className="p-2">
                <BindingRow secret={s} appId={appId} envId={envId} />
              </td>
              <td className="p-2">
                {s.selected_version}/{s.latest_version}
              </td>
              <td className="p-2">
                <RotateButton secret={s} appId={appId} envId={envId} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Draft (v{draft.data?.version ?? "…"})</h2>
        <ul className="mt-2 space-y-1 text-sm">
          {draft.data?.selections.map((sel) => (
            <li key={sel.secret_id} className="flex items-center gap-2">
              <span className="font-mono">{sel.name}</span>
              <span className="opacity-60">seq {sel.version_seq}</span>
            </li>
          ))}
        </ul>
        <button
          className="mt-3 rounded bg-amber-500 px-4 py-1 font-semibold disabled:opacity-50"
          disabled={publish.isPending}
          onClick={() => publish.mutate()}
        >
          Publish revision
        </button>
        {publish.isError && (
          <p className="mt-1 text-sm text-red-500">
            {String((publish.error as Error).message)}
          </p>
        )}
        {publish.isSuccess && (
          <p className="mt-1 text-sm text-green-600">Published {publish.data?.id}</p>
        )}
      </section>

      <section className="rounded border p-4">
        <h2 className="font-semibold">Revisions</h2>
        <ul className="mt-2 space-y-1 text-sm">
          {revisions.data?.map((r) => (
            <li key={r.id} className="font-mono">
              {r.id.slice(0, 8)}… · draft v{r.draft_version} · {r.file_count} files · by {r.created_by}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

export function AppPage() {
  const { appId } = useParams();
  const app = useApplication(appId!);
  const createEnv = useCreateEnvironment(appId!);
  const [envName, setEnvName] = useState("");
  const [activeEnv, setActiveEnv] = useState<string | null>(null);

  if (app.isLoading) return <p>Loading…</p>;
  if (app.isError) return <p className="text-red-500">Application not found</p>;

  return (
    <div className="space-y-4">
      <Link to="/apps" className="text-sm opacity-70">← Applications</Link>
      <h1 className="text-xl font-bold">{app.data?.name}</h1>
      <div className="flex flex-wrap items-center gap-2">
        {app.data?.environments.map((env) => (
          <button
            key={env.id}
            className={`rounded px-3 py-1 ${activeEnv === env.id ? "bg-amber-500" : "border"}`}
            onClick={() => setActiveEnv(env.id)}
            data-testid={`env-${env.name}`}
          >
            {env.name}
          </button>
        ))}
        <form
          className="flex gap-1"
          onSubmit={(e) => {
            e.preventDefault();
            if (envName.trim()) {
              createEnv.mutate(envName.trim());
              setEnvName("");
            }
          }}
        >
          <input className="rounded border p-1" placeholder="new env"
            value={envName} onChange={(e) => setEnvName(e.target.value)} />
          <button className="rounded border px-2">+</button>
        </form>
      </div>
      {activeEnv ? (
        <SecretEditor appId={appId!} envId={activeEnv} />
      ) : (
        <p className="opacity-60">Select an environment to edit its secrets.</p>
      )}
    </div>
  );
}
