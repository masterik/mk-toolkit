/** Git helper utilities. */

async function run(args: string[]): Promise<string> {
  const proc = Bun.spawn(["git", ...args], {
    stdout: "pipe",
    stderr: "pipe",
  });
  const text = await new Response(proc.stdout).text();
  await proc.exited;
  return text.trim();
}

export async function getBranch(): Promise<string> {
  return run(["rev-parse", "--abbrev-ref", "HEAD"]);
}

export async function getStagedCount(): Promise<number> {
  const out = await run(["diff", "--cached", "--name-only"]);
  if (!out) return 0;
  return out.split("\n").length;
}

export async function getChangedCount(): Promise<number> {
  const out = await run(["status", "--porcelain"]);
  if (!out) return 0;
  return out.split("\n").length;
}

export async function getLastCommitOneline(): Promise<string> {
  return run(["log", "-1", "--oneline"]);
}

export async function getLastDiffStat(): Promise<string> {
  const out = await run(["diff", "HEAD~1", "--stat"]);
  const lines = out.split("\n").filter(Boolean);
  return lines.length > 0 ? lines[lines.length - 1] : "";
}
