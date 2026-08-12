import { useMutation, useQueryClient } from "@tanstack/react-query";
import { API_PATHS } from "../../lib/constants/api-paths";
import { apiPost } from "../../lib/api";
import { type MFAVerifyForm } from "../../lib/constants/schemas";

export interface BootstrapEnrollment {
  id: string;
  username: string;
  status: "pending_mfa";
  enrollment_token: string;
  totp_uri: string;
}

export interface MFAVerified {
  confirmation_token: string;
  recovery_codes: string[];
}

export function useVerifyMFAEnrollment() {
  return useMutation({
    mutationFn: (body: { enrollment_token: string } & MFAVerifyForm) =>
      apiPost<MFAVerified>(API_PATHS.mfaVerify, body),
  });
}

export function useConfirmMFAEnrollment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { confirmation_token: string }) =>
      apiPost(API_PATHS.mfaConfirm, body),
    onSuccess: () => qc.setQueryData(["me"], { bootstrap_required: false }),
  });
}

export function useStepUp() {
  return useMutation({
    mutationFn: (body: { password: string }) => apiPost(API_PATHS.stepUp, body),
  });
}
