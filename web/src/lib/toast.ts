import { toastManager } from "@/components/ui/toast";

export const SUCCESS_TOAST_ID = "success";

export function toastSuccess(title: string): void {
  toastManager.add({
    id: SUCCESS_TOAST_ID,
    title,
    type: "success",
  });
}

export function toastError(title: string): void {
  toastManager.add({
    id: SUCCESS_TOAST_ID,
    title,
    type: "error",
  });
}
