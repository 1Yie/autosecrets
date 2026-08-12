import type { FieldErrors, UseFormRegister } from "react-hook-form";
import type { OperationReasonForm } from "../lib/constants/schemas";

const categoryLabels: Record<OperationReasonForm["category"], string> = {
  maintenance: "维护",
  incident_response: "故障响应",
  access_change: "权限变更",
  configuration_correction: "配置修正",
  other: "其他",
};

interface OperationReasonFieldsProps {
  register: UseFormRegister<OperationReasonForm>;
  errors: FieldErrors<OperationReasonForm>;
}

/** Shared Operation Reason fields for Publish and Rollback (10-500 char
 * explanation, stable category, optional external reference). */
export function OperationReasonFields({ register, errors }: OperationReasonFieldsProps) {
  return (
    <div className="space-y-2">
      <div>
        <label className="text-sm">原因类别</label>
        <select className="w-full rounded border px-2 py-1 text-sm" data-testid="reason-category" {...register("category")}>
          {(Object.keys(categoryLabels) as OperationReasonForm["category"][]).map((category) => (
            <option key={category} value={category}>
              {categoryLabels[category]}
            </option>
          ))}
        </select>
        {errors.category && <p className="text-sm text-red-500">{errors.category.message}</p>}
      </div>
      <div>
        <label className="text-sm">说明（10–500 字符）</label>
        <textarea
          className="w-full rounded border px-2 py-1 text-sm"
          rows={2}
          data-testid="reason-explanation"
          {...register("explanation")}
        />
        {errors.explanation && <p className="text-sm text-red-500">{errors.explanation.message}</p>}
      </div>
      <div>
        <label className="text-sm">外部引用（可选）</label>
        <input
          className="w-full rounded border px-2 py-1 text-sm"
          data-testid="reason-external-ref"
          placeholder="例如 INC-1234"
          {...register("external_ref")}
        />
        {errors.external_ref && <p className="text-sm text-red-500">{errors.external_ref.message}</p>}
      </div>
    </div>
  );
}
