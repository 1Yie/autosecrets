import { useCursorPage } from "../shared/use-cursor-page";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface Assignment {
  id: string;
  group_id: string;
  group_name: string;
  application_id: string;
  environment_id: string;
  revision_id: string;
  status: string;
  created_at: string;
}

export function useAssignments() {
  return useCursorPage<Assignment>(["assignments"], (cursor) =>
    `${API_PATHS.assignments}?${cursor ? `cursor=${cursor}` : ""}`,
  );
}
