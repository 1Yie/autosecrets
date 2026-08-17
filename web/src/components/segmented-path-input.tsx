import { Fragment, useLayoutEffect, useRef, useState } from "react";
import { Plus } from "lucide-react";

interface SegmentedPathInputProps {
	value: string;
	onChange: (value: string) => void;
	onBlur?: () => void;
	name?: string;
	/** Base test id; the filename (last) segment gets exactly this id, directory
	 * segments get `<testId>-dir-<index>`. Insert buttons use `<testId>-insert-<index>`. */
	testId?: string;
}

function splitSegments(value: string): string[] {
	return value === "" ? [""] : value.split("/");
}

interface SegmentInputProps {
	value: string;
	placeholder: string;
	autoFocus?: boolean;
	onChange: (value: string) => void;
	onBlur: () => void;
	onFocused?: () => void;
	name?: string;
	testId?: string;
}

/** A single path pill. Its width is measured from a hidden mirror of the exact
 * rendered text, so CJK placeholders ("目录" / "文件名") and any mixed-width
 * content are never clipped. */
function SegmentInput({
	value,
	placeholder,
	autoFocus,
	onChange,
	onBlur,
	onFocused,
	name,
	testId,
}: SegmentInputProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const mirrorRef = useRef<HTMLSpanElement>(null);
	const [textWidth, setTextWidth] = useState(0);
	const text = value || placeholder;

	useLayoutEffect(() => {
		if (mirrorRef.current) setTextWidth(mirrorRef.current.offsetWidth);
	}, [text]);

	useLayoutEffect(() => {
		if (!autoFocus) return;
		inputRef.current?.focus();
		onFocused?.();
	}, [autoFocus, onFocused]);

	return (
		<>
			<span
				ref={mirrorRef}
				aria-hidden
				className="invisible fixed top-0 left-0 whitespace-nowrap font-mono text-xs"
			>
				{text}
			</span>
			<input
				ref={inputRef}
				type="text"
				className="h-7 max-w-full min-w-16 rounded-full border border-input bg-background px-2.5 font-mono text-xs text-foreground outline-none ring-ring/24 transition-shadow dark:bg-input/32 focus-visible:border-ring focus-visible:ring-[3px]"
				style={{ width: `${textWidth + 24}px` }}
				value={value}
				placeholder={placeholder}
				onChange={(event) => onChange(event.target.value)}
				onBlur={onBlur}
				name={name}
				data-testid={testId}
			/>
		</>
	);
}

function InsertButton({
	label,
	onClick,
	testId,
}: {
	label: string;
	onClick: () => void;
	testId?: string;
}) {
	return (
		<button
			type="button"
			className="flex size-5 shrink-0 items-center justify-center rounded-full border border-dashed text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
			aria-label={label}
			data-testid={testId}
			onMouseDown={(event) => event.preventDefault()}
			onClick={onClick}
		>
			<Plus aria-hidden="true" className="size-3" />
		</button>
	);
}

/** Presents a binding path (`aaa/ccc`) as editable pills joined by `/`.
 * A `+` sits before the first pill and after every pill, so a level can be
 * inserted at the start, after any existing segment, or at the end. Clearing a
 * pill deletes that level on blur. */
export function SegmentedPathInput({
	value,
	onChange,
	onBlur,
	name,
	testId,
}: SegmentedPathInputProps) {
	const segments = splitSegments(value);
	const [focusIndex, setFocusIndex] = useState<number | null>(null);

	const update = (index: number, raw: string) => {
		const next = segments.slice();
		next[index] = raw.replace(/\//g, "");
		onChange(next.join("/"));
	};

	const insertAt = (index: number) => {
		const next = segments.slice();
		next.splice(index, 0, "");
		setFocusIndex(index);
		onChange(next.join("/"));
	};

	const removeIfEmpty = (index: number) => {
		if (segments[index] !== "") {
			onBlur?.();
			return;
		}
		const next = segments.filter((_, i) => i !== index);
		onChange(next.length ? next.join("/") : "");
		setFocusIndex((current) => {
			if (current === null) return current;
			if (current === index) return null;
			return current > index ? current - 1 : current;
		});
		onBlur?.();
	};

	const insertTestId = (index: number) =>
		testId ? `${testId}-insert-${index}` : undefined;

	return (
		<div className="flex flex-wrap items-center gap-1">
			<InsertButton
				label="在开头添加层级"
				testId={insertTestId(0)}
				onClick={() => insertAt(0)}
			/>
			{segments.map((seg, i) => {
				const isFilename = i === segments.length - 1;
				const segTestId = testId
					? isFilename
						? testId
						: `${testId}-dir-${i}`
					: undefined;
				const afterLabel = seg ? `在 ${seg} 后添加层级` : "在此段后添加层级";
				return (
					<Fragment key={i}>
						{i > 0 && <span className="text-xs text-muted-foreground">/</span>}
						<SegmentInput
							value={seg}
							placeholder={isFilename ? "文件名" : "目录"}
							autoFocus={focusIndex === i}
							onChange={(v) => update(i, v)}
							onBlur={() => removeIfEmpty(i)}
							onFocused={() => setFocusIndex(null)}
							name={name}
							testId={segTestId}
						/>
						<InsertButton
							label={afterLabel}
							testId={insertTestId(i + 1)}
							onClick={() => insertAt(i + 1)}
						/>
					</Fragment>
				);
			})}
		</div>
	);
}
