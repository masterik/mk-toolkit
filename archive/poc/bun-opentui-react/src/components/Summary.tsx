import { TextAttributes } from "@opentui/core";

interface SummaryProps {
  exitCode: number;
  logLine: string;
  diffStat: string;
}

export function Summary({ exitCode, logLine, diffStat }: SummaryProps) {
  if (exitCode === 0) {
    return (
      <box border borderStyle="rounded" borderColor="#4caf50" title="✅ Committed!" width={55} marginTop={1}>
        <text attributes={TextAttributes.BOLD}>{logLine}</text>
        <text attributes={TextAttributes.DIM}>{diffStat}</text>
      </box>
    );
  }

  return (
    <box border borderStyle="rounded" borderColor="#f44336" title="❌ Failed" width={55} marginTop={1}>
      <text>
        Claude exited with code <span fg="#f44336" attributes={TextAttributes.BOLD}>{String(exitCode)}</span>
      </text>
    </box>
  );
}
