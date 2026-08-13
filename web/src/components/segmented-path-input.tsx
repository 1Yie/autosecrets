import { Fragment, useLayoutEffect, useRef, useState } from "react";

interface SegmentedPathInputProps {
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  name?: string;
  /** Base test id; the filename (last) segment gets exactly this id, directory
   * segments get `<testId>-dir-<index>`. */
  testId?: string;
}

function splitSegments(value: string): string[] {
  return value === "" ? [""] : value.split("/");
}

interface SegmentInputProps {
  value: string;
  placeholder: string;
  onChange: (value: string) => void;
  onBlur: () => void;
  name?: string;
  testId?: string;
}

/** A single path pill. Its width is measured from a hidden mirror of the exact
 * rendered text, so CJK placeholders ("目录" / "文件名") and any mixed-width
 * content are never clipped. */
function SegmentInput({ value, placeholder, onChange, onBlur, name, testId }: SegmentInputProps) {
  const mirrorRef = useRef<HTMLSpanElement>(null);
  const [textWidth, setTextWidth] = useState(0);
  const text = value || placeholder;

  useLayoutEffect(() => {
    if (mirrorRef.current) setTextWidth(mirrorRef.current.offsetWidth);
  }, [text]);

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
        type="text"
        className="h-7 rounded-full border border-input bg-background px-2.5 font-mono text-xs text-foreground outline-none ring-ring/24 transition-shadow dark:bg-input/32 focus-visible:border-ring focus-visible:ring-[3px]"
        style={{ width: `${Math.min(textWidth + 24, 320)}px` }}
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

/** Presents a binding path (`A/1`) as editable pills joined by `/`. The last
 * pill is the filename, preceding pills are the directory hierarchy. Empty
 * pills are pruned on blur, so clearing a pill deletes that level; `+` inserts
 * a directory level before the filename. */
export function SegmentedPathInput({
  value,
  onChange,
  onBlur,
  name,
  testId,
}: SegmentedPathInputProps) {
  const segments = splitSegments(value);

  const update = (index: number, raw: string) => {
    const next = segments.slice();
    next[index] = raw.replace(/\//g, "");
    onChange(next.join("/"));
  };

  const addLevel = () => {
    const next = segments.slice();
    next.splice(Math.max(0, next.length - 1), 0, "");
    onChange(next.join("/"));
  };

  const commitClean = () => {
    const cleaned = segments.filter((s) => s !== "");
    onChange(cleaned.length ? cleaned.join("/") : "");
    onBlur?.();
  };

  return (
    <div className="flex flex-wrap items-center gap-1">
      {segments.map((seg, i) => {
        const isFilename = i === segments.length - 1;
        const segTestId = testId
          ? isFilename
            ? testId
            : `${testId}-dir-${i}`
          : undefined;
        return (
          <Fragment key={i}>
            {i > 0 && <span className="text-xs text-muted-foreground">/</span>}
            <SegmentInput
              value={seg}
              placeholder={isFilename ? "文件名" : "目录"}
              onChange={(v) => update(i, v)}
              onBlur={commitClean}
              name={name}
              testId={segTestId}
            />
          </Fragment>
        );
      })}
      <button
        type="button"
        className="flex h-7 items-center rounded-full border border-dashed px-2 text-xs text-muted-foreground transition-colors hover:bg-muted"
        aria-label="添加层级"
        onClick={addLevel}
      >
        +
      </button>
    </div>
  );
}
