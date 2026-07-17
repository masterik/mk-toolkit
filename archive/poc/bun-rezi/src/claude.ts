import type { App } from "@rezi-ui/core";
import type { AppState, MenuChoice } from "./types.js";
import { getLastCommitOneline, getLastDiffStat } from "./git.js";
import { SPINNER_FRAMES } from "./types.js";

const MAX_OUTPUT_LINES = 15;

/**
 * Spawn the Claude subprocess and stream output into app state.
 * choice and noteText are passed explicitly since App has no getState().
 */
export function spawnClaude(
  app: App<AppState>,
  choice: MenuChoice,
  noteText: string,
): void {
  let prompt = "/commit";
  if (choice === "note" && noteText.trim()) {
    prompt = `/commit ${noteText.trim()}`;
  }

  const proc = Bun.spawn(
    ["claude", "-p", prompt, "--model=haiku", "--dangerously-skip-permissions"],
    { stdout: "pipe", stderr: "pipe" },
  );

  // Start spinner animation
  const spinnerInterval = setInterval(() => {
    app.update((prev) => ({
      ...prev,
      spinnerFrame: (prev.spinnerFrame + 1) % SPINNER_FRAMES.length,
    }));
  }, 80);

  // Stream stdout line by line
  const reader = proc.stdout.getReader();
  let buffer = "";

  async function readStream() {
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += new TextDecoder().decode(value);
        const lines = buffer.split("\n");
        buffer = lines.pop() ?? "";
        if (lines.length > 0) {
          app.update((prev) => {
            const newLines = [...prev.outputLines, ...lines];
            return {
              ...prev,
              outputLines: newLines.slice(-MAX_OUTPUT_LINES),
            };
          });
        }
      }
      // Flush remaining buffer
      if (buffer.trim()) {
        app.update((prev) => {
          const newLines = [...prev.outputLines, buffer];
          return {
            ...prev,
            outputLines: newLines.slice(-MAX_OUTPUT_LINES),
          };
        });
      }
    } catch {
      // stream closed
    }
  }

  readStream();

  // Wait for process exit
  proc.exited.then(async (code) => {
    clearInterval(spinnerInterval);

    let commitLine = "";
    let diffStat = "";
    if (code === 0) {
      try {
        [commitLine, diffStat] = await Promise.all([
          getLastCommitOneline(),
          getLastDiffStat(),
        ]);
      } catch {
        // git commands may fail
      }
    }

    app.update((prev) => ({
      ...prev,
      phase: "summary",
      exitCode: code ?? 1,
      commitLine,
      diffStat,
    }));
  });
}
