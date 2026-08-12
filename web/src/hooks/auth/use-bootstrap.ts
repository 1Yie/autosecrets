import { useMutation } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type BootstrapForm } from "../../lib/constants/schemas";

export function useBootstrap() {
  return useMutation({
    mutationFn: (body: BootstrapForm) => apiPost(API_PATHS.bootstrap, body),
  });
}
