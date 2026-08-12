// Zod schemas for every form. Shared with tests; validation rules mirror
// Core's checks (username shape, password length, binding path rules).
import { z } from "zod";

export const usernameSchema = z
  .string()
  .min(2, "至少 2 个字符")
  .max(64, "最多 64 个字符")
  .regex(/^[a-zA-Z0-9._-]+$/, "仅允许字母、数字、._-");

export const passwordSchema = z
  .string()
  .min(12, "密码至少 12 个字符")
  .max(128, "密码最多 128 个字符");

export const bootstrapSchema = z.object({
  code: z.string().min(1, "请输入初始化码"),
  organization_name: z.string().min(1, "请输入组织名称").max(128, "组织名称最多 128 个字符"),
  username: usernameSchema,
  password: passwordSchema,
});

export const loginSchema = z.object({
  username: z.string().min(1, "请输入用户名"),
  password: z.string().min(1, "请输入密码"),
  totp_code: z.string().regex(/^\d{6}$/, "请输入 6 位动态验证码").optional().or(z.literal("")),
  recovery_code: z.string().optional().or(z.literal("")),
});

export const mfaVerifySchema = z.object({
  totp_code: z.string().regex(/^\d{6}$/, "请输入 6 位动态验证码"),
});

export const stepUpSchema = z.object({
  password: z.string().min(1, "请输入当前密码"),
});

export const operationReasonSchema = z.object({
  category: z.enum(
    ["maintenance", "incident_response", "access_change", "configuration_correction", "other"],
    { message: "请选择原因类别" },
  ),
  explanation: z
    .string()
    .min(10, "说明至少 10 个字符")
    .max(500, "说明最多 500 个字符"),
  external_ref: z.string().max(128, "外部引用最多 128 个字符").optional().or(z.literal("")),
});

export const nameSchema = z
  .string()
  .trim()
  .min(1, "名称不能为空")
  .max(64, "名称最多 64 个字符");

export const secretSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "密钥名不能为空")
    .max(128, "密钥名最多 128 个字符")
    .refine((v) => !v.includes("/") && !v.includes("\0"), "密钥名不能包含 / 或 NUL"),
  value: z.string().min(1, "密钥值不能为空"),
});

export const bindingSchema = z.object({
  path: z
    .string()
    .trim()
    .min(1, "路径不能为空")
    .refine((p) => !p.startsWith("/"), "路径必须是相对路径")
    .refine(
      (p) => !p.split("/").some((part) => part === "" || part === "." || part === ".."),
      "路径不能包含空、. 或 .. 组件",
    ),
  mode: z.enum(["0400", "0440", "0444", "0600", "0640", "0644"]),
});

export const nodeNameSchema = nameSchema;

export type BootstrapForm = z.infer<typeof bootstrapSchema>;
export type LoginForm = z.infer<typeof loginSchema>;
export type MFAVerifyForm = z.infer<typeof mfaVerifySchema>;
export type StepUpForm = z.infer<typeof stepUpSchema>;
export type OperationReasonForm = z.infer<typeof operationReasonSchema>;
export type SecretForm = z.infer<typeof secretSchema>;
export type BindingForm = z.infer<typeof bindingSchema>;
