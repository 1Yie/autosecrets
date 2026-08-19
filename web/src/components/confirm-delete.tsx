import { Button } from "./ui/button";
import {
	AlertDialog,
	AlertDialogClose,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogPopup,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "./ui/alert-dialog";

interface ConfirmDeleteProps {
	label?: string;
	confirmLabel?: string;
	title: string;
	description: string;
	pending?: boolean;
	error?: string;
	onConfirm: () => void;
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
}

export function ConfirmDelete({
	label,
	confirmLabel,
	title,
	description,
	pending,
	error,
	onConfirm,
	open,
	onOpenChange,
}: ConfirmDeleteProps) {
	return (
		<AlertDialog open={open} onOpenChange={onOpenChange}>
			{label ? (
				<AlertDialogTrigger
					render={<Button variant="destructive-outline" size="sm" />}
				>
					{label}
				</AlertDialogTrigger>
			) : null}
			<AlertDialogPopup>
				<AlertDialogHeader>
					<AlertDialogTitle>{title}</AlertDialogTitle>
					<AlertDialogDescription>{description}</AlertDialogDescription>
				</AlertDialogHeader>
				{error && <p className="px-6 text-sm text-red-500">{error}</p>}
				<AlertDialogFooter>
					<AlertDialogClose render={<Button variant="ghost" />}>
						取消
					</AlertDialogClose>
					<Button variant="destructive" loading={pending} onClick={onConfirm}>
						{confirmLabel ?? "删除"}
					</Button>
				</AlertDialogFooter>
			</AlertDialogPopup>
		</AlertDialog>
	);
}
