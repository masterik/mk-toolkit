#!/usr/bin/env bun
/**
 * ForgeZ POC A — fz commit (Bun + openTUI React)
 *
 * Exercises: styled boxes, interactive menu, spinner, subprocess streaming, summary display.
 * Run: bun run apps/cli/src/commit.tsx
 */

const startNs = Bun.nanoseconds();

import { useState, useCallback } from "react";
import { createCliRenderer, TextAttributes, type SelectOption } from "@opentui/core";
import { createRoot, useKeyboard } from "@opentui/react";

import type { GitInfo, Phase } from "./types.ts";
import { MENU_OPTIONS } from "./types.ts";
import { git, fetchGitInfo } from "./utils/git.ts";
import { Header, OutputBox, Spinner, Summary } from "./components/index.ts";
import { useSubprocess } from "./hooks/useSubprocess.ts";

// ── Main App ──────────────────────────────────────────────────────────────────

function CommitApp({ gitInfo }: { gitInfo: GitInfo }) {
  const { outputLines, phase, runClaude } = useSubprocess();
  const [noteValue, setNoteValue] = useState("");
  const [localPhase, setLocalPhase] = useState<"menu" | "note-input">("menu");

  // Subprocess phases (running/summary) take priority over local UI phase
  const activePhase: Phase = phase.kind !== "menu" ? phase : (localPhase === "note-input" ? { kind: "note-input" } : { kind: "menu" });

  const handleMenuSelect = useCallback(async (_index: number, option: SelectOption | null) => {
    if (!option) return;

    if (option.value === "all") {
      await git("add", "-A");
      runClaude("/commit");
    } else if (option.value === "staged") {
      runClaude("/commit");
    } else if (option.value === "note") {
      setLocalPhase("note-input");
    }
  }, [runClaude]);

  const handleNoteSubmit = useCallback((value: string) => {
    const note = value.trim();
    const prompt = note ? `/commit ${note}` : "/commit";
    runClaude(prompt);
  }, [runClaude]);

  // Exit on summary phase key press
  useKeyboard((key) => {
    if (activePhase.kind === "summary" && (key.name === "q" || key.name === "return")) {
      process.exit(activePhase.exitCode);
    }
  });

  return (
    <box flexDirection="column" padding={1}>
      <Header gitInfo={gitInfo} />

      {activePhase.kind === "menu" && (
        <box flexDirection="column" marginTop={1}>
          <text attributes={TextAttributes.BOLD}>How would you like to commit?</text>
          <select
            focused
            options={MENU_OPTIONS}
            selectedIndex={1}
            showDescription
            wrapSelection
            focusedTextColor="#00bcd4"
            selectedTextColor="#00bcd4"
            selectedBackgroundColor="#1a3a4a"
            onSelect={handleMenuSelect}
            marginTop={1}
          />
        </box>
      )}

      {activePhase.kind === "note-input" && (
        <box flexDirection="column" marginTop={1}>
          <text attributes={TextAttributes.BOLD}>Enter commit note:</text>
          <input
            focused
            placeholder="Describe your changes..."
            value={noteValue}
            onInput={setNoteValue}
            // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment -- openTUI's onSubmit takes string, not SubmitEvent
            onSubmit={handleNoteSubmit as never}
            marginTop={1}
          />
          <text attributes={TextAttributes.DIM}>Press enter to submit · esc to go back</text>
        </box>
      )}

      {activePhase.kind === "running" && (
        <box flexDirection="column" marginTop={1}>
          <Spinner message="Running claude commit..." />
          <OutputBox lines={outputLines} />
        </box>
      )}

      {activePhase.kind === "summary" && (
        <box flexDirection="column">
          <OutputBox lines={outputLines} />
          <Summary exitCode={activePhase.exitCode} logLine={activePhase.logLine} diffStat={activePhase.diffStat} />
          <text attributes={TextAttributes.DIM} marginTop={1}>Press q or enter to exit</text>
        </box>
      )}
    </box>
  );
}

// ── Bootstrap ─────────────────────────────────────────────────────────────────

async function main() {
  const readyMs = ((Bun.nanoseconds() - startNs) / 1_000_000).toFixed(0);
  process.stderr.write(`\x1b[2m[fz] ready in ${readyMs}ms\x1b[0m\n`);

  const gitInfo = await fetchGitInfo();

  const renderer = await createCliRenderer({ exitOnCtrlC: true });
  createRoot(renderer).render(<CommitApp gitInfo={gitInfo} />);
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
