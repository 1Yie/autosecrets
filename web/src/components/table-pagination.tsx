import { useState } from "react";
import {
	PAGE_SIZE_OPTIONS,
	visiblePageItems,
} from "../lib/constants/pagination";
import { FrameFooter } from "./ui/frame";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
	Pagination,
	PaginationContent,
	PaginationEllipsis,
	PaginationItem,
	PaginationLink,
	PaginationNext,
	PaginationPrevious,
} from "./ui/pagination";
import {
	Select,
	SelectItem,
	SelectPopup,
	SelectTrigger,
	SelectValue,
} from "./ui/select";

export interface TablePaginationPage {
	pageIndex: number;
	pageSize: number;
	pageCount: number;
	nextCursor: string;
	isFirstPage: boolean;
	next: () => void;
	prev: () => void;
	goToPage: (page: number) => void;
	setLimit: (limit: number) => void;
}

interface TablePaginationProps {
	page: TablePaginationPage;
	noun: string;
}

function TablePagination({ page, noun }: TablePaginationProps) {
	const [jump, setJump] = useState("");
	const sizeItems = PAGE_SIZE_OPTIONS.map((size) => ({
		label: String(size),
		value: size,
	}));

	return (
		<FrameFooter className="p-2">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<div className="flex flex-wrap items-center gap-2 whitespace-nowrap">
					<p className="text-muted-foreground text-sm">显示</p>
					<Select
						items={sizeItems}
						onValueChange={(value) => {
							if (typeof value === "number") page.setLimit(value);
						}}
						value={page.pageSize}
					>
						<SelectTrigger
							aria-label="每页显示数量"
							className="w-fit min-w-0"
							size="sm"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectPopup>
							{sizeItems.map((item) => (
								<SelectItem
									className="data-highlighted:bg-foreground/8"
									key={item.value}
									value={item.value}
								>
									{item.label}
								</SelectItem>
							))}
						</SelectPopup>
					</Select>
					<p className="text-muted-foreground text-sm">条{noun}</p>
				</div>

				<div className="flex items-center gap-2">
					<form
						className="flex items-center gap-1.5"
						onSubmit={(event) => {
							event.preventDefault();
							const nextPage = Number.parseInt(jump, 10);
							if (Number.isInteger(nextPage)) page.goToPage(nextPage);
						}}
					>
						<p className="text-muted-foreground text-sm">跳至</p>
						<Input
							aria-label="跳至页码"
							className="w-14"
							inputMode="numeric"
							min={1}
							onChange={(event) => setJump(event.target.value)}
							size="sm"
							type="number"
							value={jump}
						/>
						<p className="text-muted-foreground text-sm">页</p>
						<Button size="sm" type="submit" variant="outline">
							跳转
						</Button>
					</form>
					<Pagination className="mx-0 w-auto justify-end">
						<PaginationContent>
							<PaginationItem>
								<PaginationPrevious
									className="sm:*:[svg]:hidden"
									render={
										<Button
											disabled={page.isFirstPage}
											onClick={page.prev}
											size="sm"
											type="button"
											variant="outline"
										/>
									}
								/>
							</PaginationItem>
							{visiblePageItems(page.pageIndex, page.pageCount).map(
								(item, index) =>
									item === "ellipsis" ? (
										<PaginationItem key={`ellipsis-${index}`}>
											<PaginationEllipsis />
										</PaginationItem>
									) : (
										<PaginationItem key={item}>
											<PaginationLink
												isActive={item === page.pageIndex}
												render={
													<Button
														aria-label={`第 ${item} 页`}
														className={
															item === page.pageIndex
																? undefined
																: "hover:bg-foreground/8 data-pressed:bg-foreground/12"
														}
														onClick={() => page.goToPage(item)}
														size="icon"
														type="button"
														variant={
															item === page.pageIndex ? "outline" : "ghost"
														}
													/>
												}
											>
												{item}
											</PaginationLink>
										</PaginationItem>
									),
							)}
							<PaginationItem>
								<PaginationNext
									className="sm:*:[svg]:hidden"
									render={
										<Button
											disabled={
												!page.nextCursor && page.pageIndex >= page.pageCount
											}
											onClick={page.next}
											size="sm"
											type="button"
											variant="outline"
										/>
									}
								/>
							</PaginationItem>
						</PaginationContent>
					</Pagination>
				</div>
			</div>
		</FrameFooter>
	);
}

export { TablePagination };
export default TablePagination;
