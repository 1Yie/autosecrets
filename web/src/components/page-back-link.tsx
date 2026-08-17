import { Link } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { cn } from "@/lib/utils";

interface PageBackLinkProps {
	to: string;
	children: React.ReactNode;
	className?: string;
}

/** Compact parent-page link used at the top of detail screens. */
export function PageBackLink({ to, children, className }: PageBackLinkProps) {
	return (
		<Link
			to={to}
			className={cn(
				"inline-flex items-center gap-1 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline",
				className,
			)}
		>
			<ChevronLeft aria-hidden="true" className="size-3.5" />
			{children}
		</Link>
	);
}
