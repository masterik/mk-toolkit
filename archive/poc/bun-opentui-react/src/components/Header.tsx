import { TextAttributes } from "@opentui/core";
import type { GitInfo } from "../types.ts";

export function Header({ gitInfo }: { gitInfo: GitInfo }) {
  const { branch, stagedCount, totalCount } = gitInfo;
  const count = stagedCount > 0 ? stagedCount : totalCount;
  const label = stagedCount > 0 ? "staged" : "changed";

  return (
    <box border borderStyle="rounded" borderColor="#00bcd4" padding={0} width={44}>
      <text>
        <b>⚡ ForgeZ</b> — commit
      </text>
      <text>
        📍 <span fg="#00bcd4" attributes={TextAttributes.BOLD}>{branch}</span>
        {" · "}{count} files {label}
      </text>
    </box>
  );
}
