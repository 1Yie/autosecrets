import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TablePagination } from "../../components/table-pagination";

function page(
	overrides: Partial<Parameters<typeof TablePagination>[0]["page"]> = {},
) {
	return {
		pageIndex: 1,
		pageSize: 25,
		pageCount: 3,
		nextCursor: "next",
		isFirstPage: true,
		next: vi.fn(),
		prev: vi.fn(),
		goToPage: vi.fn(),
		setLimit: vi.fn(),
		...overrides,
	};
}

describe("TablePagination", () => {
	it("shows the page size, jump field, and numbered pager", () => {
		render(<TablePagination noun="应用" page={page()} />);
		expect(screen.getByLabelText("每页显示数量")).toBeInTheDocument();
		expect(screen.queryByLabelText("当前显示范围")).not.toBeInTheDocument();
		expect(screen.getByLabelText("跳至页码")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "跳转" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
		expect(screen.getByRole("button", { name: "第 1 页" })).toHaveAttribute(
			"aria-current",
			"page",
		);
		expect(screen.getByRole("button", { name: "第 2 页" })).toBeEnabled();
		expect(screen.getByRole("button", { name: "第 2 页" })).toHaveClass(
			"hover:bg-foreground/8",
		);
		expect(screen.getByRole("button", { name: "第 3 页" })).toBeEnabled();
		expect(screen.getByRole("button", { name: "下一页" })).toBeEnabled();
		expect(screen.getByRole("button", { name: "第 1 页" })).not.toHaveClass(
			"hover:bg-foreground/8",
		);
	});

	it("jumps to the typed page", async () => {
		const current = page();
		const user = userEvent.setup();
		render(<TablePagination noun="应用" page={current} />);
		await user.type(screen.getByLabelText("跳至页码"), "2");
		await user.click(screen.getByRole("button", { name: "跳转" }));
		expect(current.goToPage).toHaveBeenCalledWith(2);
	});

	it("jumps to a numbered page", async () => {
		const current = page({ pageIndex: 2, isFirstPage: false });
		const user = userEvent.setup();
		render(<TablePagination noun="应用" page={current} />);
		await user.click(screen.getByRole("button", { name: "第 3 页" }));
		expect(current.goToPage).toHaveBeenCalledWith(3);
	});

	it("collapses a long page range with ellipsis", () => {
		render(
			<TablePagination
				noun="应用"
				page={page({ pageIndex: 5, pageCount: 12, isFirstPage: false })}
			/>,
		);
		expect(screen.getByRole("button", { name: "第 1 页" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "第 4 页" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "第 5 页" })).toHaveAttribute(
			"aria-current",
			"page",
		);
		expect(screen.getByRole("button", { name: "第 6 页" })).toBeInTheDocument();
		expect(
			screen.getByRole("button", { name: "第 12 页" }),
		).toBeInTheDocument();
		expect(screen.getAllByText("更多页")).toHaveLength(2);
	});
});
