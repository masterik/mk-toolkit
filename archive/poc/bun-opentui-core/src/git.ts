/** Git helper utilities for fz commit. */

export interface GitInfo {
  readonly branch: string;
  readonly stagedCount: number;
  readonly totalCount: number;
}

/** Run a git command and return trimmed stdout. */
export async function git(...args: string[]): Promise<string> {
  const proc = Bun.spawn(["git", ...args], { stdout: "pipe", stderr: "pipe" });
  const text = await new Response(proc.stdout).text();
  await proc.exited;
  return text.trim();
}

/** Gather branch, staged count, and total changed count in parallel. */
export async function fetchGitInfo(): Promise<GitInfo> {
  const [branch, stagedDiff, statusOutput] = await Promise.all([
    git("rev-parse", "--abbrev-ref", "HEAD"),
    git("diff", "--cached", "--name-only"),
    git("status", "--porcelain"),
  ]);

  return {
    branch,
    stagedCount: stagedDiff ? stagedDiff.split("\n").length : 0,
    totalCount: statusOutput ? statusOutput.split("\n").length : 0,
  };
}
