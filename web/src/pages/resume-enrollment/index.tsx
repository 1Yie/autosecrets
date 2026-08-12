import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useResumeMFAEnrollment } from "../../hooks/auth/use-resume-mfa";
import { MFAEnrollmentSteps, type EnrollmentContext } from "../../components/mfa-enrollment-steps";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { z } from "zod";

const resumeSchema = z.object({
  username: z.string().min(1, "请输入用户名"),
  password: z.string().min(1, "请输入密码"),
});

/** Interrupted-Bootstrap recovery: a pending first Administrator proves the
 * password to receive a fresh enrollment token, then completes TOTP and
 * Recovery Code confirmation exactly like the initial flow (US-31). */
export function ResumeEnrollmentPage() {
  const resume = useResumeMFAEnrollment();
  const [enrollment, setEnrollment] = useState<EnrollmentContext | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isValid },
  } = useForm<{ username: string; password: string }>({
    resolver: zodResolver(resumeSchema),
    mode: "onChange",
  });

  if (enrollment) {
    return (
      <MFAEnrollmentSteps
        enrollment={enrollment}
        onDone={() => setEnrollment(null)}
      />
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">完成首次管理员注册</h1>
      <p className="text-sm opacity-70">
        首次注册已开始但未完成。输入之前设置的用户名与密码，重新获取 TOTP
        绑定与一次性恢复码；注册完成后即可登录。
      </p>
      <form
        onSubmit={handleSubmit((values) =>
          resume.mutate(values, { onSuccess: setEnrollment }),
        )}
        className="space-y-3"
        data-testid="resume-form"
      >
        <Input className="w-full" placeholder="用户名" data-testid="username" {...register("username")} />
        {errors.username && <p className="text-sm text-red-500">{errors.username.message}</p>}
        <Input
          className="w-full"
          type="password"
          placeholder="密码"
          data-testid="password"
          {...register("password")}
        />
        {errors.password && <p className="text-sm text-red-500">{errors.password.message}</p>}
        {resume.isError && (
          <p className="text-sm text-red-500">{String((resume.error as Error).message)}</p>
        )}
        <Button
          variant="default"
          className="w-full"
          disabled={!isValid || resume.isPending}
          type="submit"
        >
          继续注册
        </Button>
      </form>
    </div>
  );
}

export default ResumeEnrollmentPage;
