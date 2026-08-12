import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../../lib/api";
import { API_PATHS } from "../../lib/constants/api-paths";

export interface Assignment {
  id: string;
  group_id: string;
  group_name: string;
  revision_id: string;
  created_at: string;
}

export function useAssignments() {
  return useQuery({
    queryKey: ["assignments"],
    queryFn: () => apiGet<Assignment[]>(API_PATHS.assignments),
  });
}
