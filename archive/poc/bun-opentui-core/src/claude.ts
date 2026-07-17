/** Subprocess management for spawning Claude. */

import { git } from "./git.ts";

export interface SubprocessCallbacks {
  onLine: (line: string) => void;
  onDone: (result: SubprocessResult) => void;
}

export interface SubprocessResult {
  exitCode: number;
  logLine: string;
  diffStat: string;
}

/**
 * Spawn Claude subprocess, stream stdout line by line,
 * then gather git summary on completion.
 */
export function runClaude(prompt: string, callbacks: SubprocessCallbacks): void {
  const args = ["claude", "-p", prompt, "--model=haiku", "--dangerously-skip-permissions"];
  const proc = Bun.spawn(args, { stdout: "pipe", stderr: "pipe" });

  const reader = proc.stdout.getReader();
  const decoder = new TextDecoder();

  const run = async () => {
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true });
        const lines = chunk.split("\n").filter(Boolean);
        for (const line of lines) {
          callbacks.onLine(line);
        }
      }
    } catch {
      // stream closed
    }

    const code = await proc.exited;

    let logLine = "";
    let diffStat = "";

    if (code === 0) {
      try {
        const log = await git("log", "-1", "--oneline");
        const stat = await git("diff", "HEAD~1", "--stat", "--no-color");
        const statLines = stat.split("\n").filter(Boolean);
        logLine = log;
        diffStat = statLines[statLines.length - 1]?.trim() ?? "";
      } catch {
        logLine = "(committed)";
      }
    }

    callbacks.onDone({ exitCode: code, logLine, diffStat });
  };

  run().catch(() => {
    callbacks.onDone({ exitCode: 1, logLine: "", diffStat: "" });
  });
}
