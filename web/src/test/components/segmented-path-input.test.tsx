import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { SegmentedPathInput } from "../../components/segmented-path-input";

function Harness({ initial = "aaa/ccc" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <div>
      <SegmentedPathInput value={value} onChange={setValue} testId="path" />
      <output data-testid="joined">{value}</output>
    </div>
  );
}

describe("SegmentedPathInput", () => {
  it("inserts a level before the first segment", async () => {
    render(<Harness />);
    const user = userEvent.setup();
    await user.click(screen.getByTestId("path-insert-0"));
    await user.type(document.activeElement as HTMLElement, "root");
    expect(screen.getByTestId("joined")).toHaveTextContent("root/aaa/ccc");
    expect((screen.getByTestId("path-dir-0") as HTMLInputElement).value).toBe("root");
    expect((screen.getByTestId("path-dir-1") as HTMLInputElement).value).toBe("aaa");
    expect((screen.getByTestId("path") as HTMLInputElement).value).toBe("ccc");
  });

  it("inserts a level after the last segment", async () => {
    render(<Harness />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "在 ccc 后添加层级" }));
    await user.type(document.activeElement as HTMLElement, "file");
    expect(screen.getByTestId("joined")).toHaveTextContent("aaa/ccc/file");
    expect((screen.getByTestId("path-dir-0") as HTMLInputElement).value).toBe("aaa");
    expect((screen.getByTestId("path-dir-1") as HTMLInputElement).value).toBe("ccc");
    expect((screen.getByTestId("path") as HTMLInputElement).value).toBe("file");
  });

  it("inserts a level after an intermediate segment", async () => {
    render(<Harness />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "在 aaa 后添加层级" }));
    await user.type(document.activeElement as HTMLElement, "bbb");
    expect(screen.getByTestId("joined")).toHaveTextContent("aaa/bbb/ccc");
  });
});
