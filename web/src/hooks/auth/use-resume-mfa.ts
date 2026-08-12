import { useMutation } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import type { EnrollmentContext } from "../../components/mfa-enrollment-steps";

export function useResumeMFAEnrollment() {
  return useMutation({
    mutationFn: (body: { username: string; password: string }) =>
      apiPost<EnrollmentContext>(`${API_PATHS.mfaResume}`, body),
  });
}
