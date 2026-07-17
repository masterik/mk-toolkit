import { TextAttributes } from "@opentui/core";

const MAX_VISIBLE_LINES = 15;

export function OutputBox({ lines }: { lines: string[] }) {
  if (lines.length === 0) return null;

  const visible = lines.slice(-MAX_VISIBLE_LINES);

  return (
    <box border borderStyle="single" borderColor="#555555" title="claude output" width={60} marginTop={1}>
      {visible.map((line, i) => (
        <text key={i} attributes={TextAttributes.DIM}>{line}</text>
      ))}
    </box>
  );
}
