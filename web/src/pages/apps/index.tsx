import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "react-router-dom";
import { useApplications } from "../../hooks/applications/use-applications";
import { useCreateApplication } from "../../hooks/applications/use-create-application";
import { nameSchema } from "../../lib/constants/schemas";
import { z } from "zod";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

const createAppSchema = z.object({ name: nameSchema });

export function AppsPage() {
  const apps = useApplications();
  const create = useCreateApplication();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<{ name: string }>({ resolver: zodResolver(createAppSchema) });

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
        onSubmit={handleSubmit((v) => {
          create.mutate(v.name);
          reset();
        })}
      >
        <Input className="flex-1"
          placeholder="Application name"
          data-testid="app-name"
          {...register("name")} />
        {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
        <Button variant="default"  type="submit">
          Create
        </Button>
      </form>
    </div>
  );
}


export default AppsPage;
