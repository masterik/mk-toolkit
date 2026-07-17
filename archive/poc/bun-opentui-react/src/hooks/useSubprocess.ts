import { useState, useCallback, useRef } from "react";
import { git } from "../utils/git.ts";
import type { Phase } from "../types.ts";

interface SubprocessResult {
  outputLines: string[];
  phase: Phase;
  runClaude: (prompt: string) => void;
}

/**
 * Hook that manages spawning a Claude subprocess, streaming its output,
 * and transitioning to the summary phase when done.
 */
export function useSubprocess(): SubprocessResult {
  const [outputLines, setOutputLines] = useState<string[]>([]);
  const [phase, setPhase] = useState<Phase>({ kind: "menu" });
  const runningRef = useRef(false);

  const runClaude = useCallback((prompt: string) => {
    if (runningRef.current) return;
    runningRef.current = true;

    setPhase({ kind: "running", prompt });
    setOutputLines([]);

    const run = async () => {
      const args = ["claude", "-p", prompt, "--model=haiku", "--dangerously-skip-permissions"];
      const proc = Bun.spawn(args, { stdout: "pipe", stderr: "pipe" });

      const reader = proc.stdout.getReader();
      const decoder = new TextDecoder();

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          const chunk = decoder.decode(value, { stream: true });
          const lines = chunk.split("\n").filter(Boolean);
          if (lines.length > 0) {
            setOutputLines((prev: string[]) => [...prev, ...lines]);
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

      runningRef.current = false;
      setPhase({ kind: "summary", exitCode: code, logLine, diffStat });
    };

    run().catch(() => {
      runningRef.current = false;
      setPhase({ kind: "summary", exitCode: 1, logLine: "", diffStat: "" });
    });
  }, []);

  return { outputLines, phase, runClaude };
}
