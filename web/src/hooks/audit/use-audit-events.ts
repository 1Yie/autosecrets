import { useState } from "react";
import { API_PATHS } from "../../lib/constants/api-paths";
import { useCursorPage } from "../shared/use-cursor-page";

export interface AuditEvent {
	id: number;
	actor: string;
	action: string;
	resource: string;
	result: string;
	correlation_id: string;
	created_at: string;
	actor_type: string;
	actor_id: string;
	actor_display: string;
	resource_type: string;
	resource_id: string;
	resource_display: string;
	outcome: string;
	operation_reason_category: string;
	operation_reason_explanation: string;
	operation_reason_external_ref: string;
}

export interface AuditFilters {
	actor: string;
	action: string;
	resource: string;
	outcome: string;
	reason_category: string;
}

export const emptyAuditFilters: AuditFilters = {
	actor: "",
	action: "",
	resource: "",
	outcome: "",
	reason_category: "",
};

export function useAuditEvents() {
	const [filters, setFilters] = useState<AuditFilters>(emptyAuditFilters);
	const page = useCursorPage<AuditEvent>(
		["audit-events", filters],
		(cursor, page) => {
			const params = new URLSearchParams();
			if (cursor) params.set("cursor", cursor);
			else if (page > 1) params.set("page", String(page));
			for (const [key, value] of Object.entries(filters)) {
				if (value) params.set(key, value);
			}
			return `${API_PATHS.auditEvents}?${params.toString()}`;
		},
		{ query: { refetchInterval: 30_000 } },
	);
	const applyFilters = (nextFilters: AuditFilters) => {
		setFilters(nextFilters);
		page.reset();
	};
	return { ...page, filters, applyFilters };
}
