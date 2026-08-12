import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useLogin } from "../../hooks/auth/use-login";
import { loginSchema, type LoginForm } from "../../lib/constants/schemas";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

export function LoginPage() {
  const login = useLogin();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>({ resolver: zodResolver(loginSchema) });

  return (
    <div className="mx-auto mt-16 max-w-md space-y-4 rounded-lg border p-6">
      <h1 className="text-xl font-bold">Sign in</h1>
      <form onSubmit={handleSubmit((v) => login.mutate(v))} className="space-y-3">
        <Input className="w-full"
          placeholder="Username"
          data-testid="username"
          {...register("username")} />
        {errors.username && <p className="text-sm text-red-500">{errors.username.message}</p>}
        <Input className="w-full"
          type="password"
          placeholder="Password"
          data-testid="password"
          {...register("password")} />
        {errors.password && <p className="text-sm text-red-500">{errors.password.message}</p>}
        {login.isError && (
          <p className="text-sm text-red-500">
            {String((login.error as Error).message)}
          </p>
        )}
        <Button variant="default" className="w-full"
          
          disabled={isSubmitting || login.isPending}
          type="submit"
        >
          Sign in
        </Button>
      </form>
    </div>
  );
}
