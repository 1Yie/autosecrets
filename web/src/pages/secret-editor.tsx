import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSecrets } from "../hooks/applications/use-secrets";
import { useCreateSecret } from "../hooks/applications/use-create-secret";
import { useDraft } from "../hooks/applications/use-draft";
import { usePublish } from "../hooks/applications/use-publish";
import { useRevisions } from "../hooks/applications/use-revisions";
import { secretSchema, type SecretForm } from "../lib/constants/schemas";
import { BindingRow } from "./binding-row";
import { RotateButton } from "./rotate-button";
import { DraftPanel } from "./draft-panel";
import { RevisionsPanel } from "./revisions-panel";
import { ErrorBoundary } from "../components/error-boundary";

interface SecretEditorProps {
  appId: string;
  envId: string;
}

export function SecretEditor({ appId, envId }: SecretEditorProps) {
  const secrets = useSecrets(appId, envId);
  const create = useCreateSecret(appId, envId);
  const draft = useDraft(appId, envId);
  const publish = usePublish(appId, envId);
  const revisions = useRevisions(appId, envId);
  const { register, handleSubmit, reset, formState: { errors } } =
    useForm<SecretForm>({ resolver: zodResolver(secretSchema) });

  return (
    <div className="space-y-6">
      <form
        className="flex gap-2"
        onSubmit={handleSubmit((v) => {
          create.mutate(v);
          reset();
        })}
      >
        <input className="flex-1 rounded border p-2" placeholder="Secret name"
          data-testid="secret-name" {...register("name")} />
        <input className="flex-1 rounded border p-2" placeholder="Secret value"
          data-testid="secret-value" {...register("value")} />
        <button className="rounded bg-amber-500 px-4 font-semibold" type="submit">
          Add secret
        </button>
      </form>
      {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
      {errors.value && <p className="text-sm text-red-500">{errors.value.message}</p>}

      {secrets.isLoading && <p>Loading secrets…</p>}
      {secrets.isError && <p className="text-red-500">Failed to load secrets</p>}
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

      <ErrorBoundary>
        <DraftPanel draft={draft} publish={publish} />
      </ErrorBoundary>
      <ErrorBoundary>
        <RevisionsPanel revisions={revisions} />
      </ErrorBoundary>
    </div>
  );
}
