/**
 * Imperative TUI rendering using @opentui/core directly (no React).
 *
 * Each phase is a function that builds/replaces renderables on the root.
 * State transitions call clearRoot() then build the new phase's tree.
 */

import {
  type CliRenderer,
  TextAttributes,
  type SelectOption,
  BoxRenderable,
  TextRenderable,
  SelectRenderable,
  SelectRenderableEvents,
  InputRenderable,
  InputRenderableEvents,
  type KeyEvent,
} from "@opentui/core";

import type { GitInfo } from "./git.ts";
import { git } from "./git.ts";
import { runClaude, type SubprocessResult } from "./claude.ts";

// ── Constants ────────────────────────────────────────────────────────────────

const CYAN = "#00bcd4";
const GRAY = "#555555";
const GREEN = "#4caf50";
const RED = "#f44336";
const MAX_OUTPUT_LINES = 15;

const BRAILLE = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"] as const;

const MENU_OPTIONS: SelectOption[] = [
  { name: "\u{1F680} Commit all", description: "stage everything + commit", value: "all" },
  { name: "\u{1F4E6} Commit staged only", description: "spawn Claude as-is", value: "staged" },
  { name: "\u{1F4DD} Custom note", description: "pass message to Claude", value: "note" },
];

// ── App State ────────────────────────────────────────────────────────────────

type Phase = "menu" | "note-input" | "running" | "summary";

interface AppState {
  renderer: CliRenderer;
  gitInfo: GitInfo;
  phase: Phase;
  outputLines: string[];
  summaryResult: SubprocessResult | null;
  noteValue: string;
  spinnerFrame: number;
  spinnerTimer: ReturnType<typeof setInterval> | null;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function clearRoot(state: AppState): void {
  const root = state.renderer.root;
  for (const child of root.getChildren()) {
    root.remove(child.id);
    child.destroyRecursively();
  }
}

function createText(
  ctx: CliRenderer,
  content: string,
  options: {
    fg?: string;
    attributes?: number;
    marginTop?: number;
  } = {},
): TextRenderable {
  const text = new TextRenderable(ctx, {
    fg: options.fg,
    attributes: options.attributes,
    marginTop: options.marginTop,
  });
  text.content = content;
  return text;
}

// ── Header (shared across phases) ────────────────────────────────────────────

function buildHeader(state: AppState): BoxRenderable {
  const { gitInfo, renderer } = state;
  const count = gitInfo.stagedCount > 0 ? gitInfo.stagedCount : gitInfo.totalCount;
  const label = gitInfo.stagedCount > 0 ? "staged" : "changed";

  const headerBox = new BoxRenderable(renderer, {
    border: true,
    borderStyle: "rounded",
    borderColor: CYAN,
    padding: 0,
    width: 44,
    flexDirection: "column",
  });

  const titleText = createText(renderer, "\u26A1 ForgeZ \u2014 commit", {
    attributes: TextAttributes.BOLD,
  });
  headerBox.add(titleText);

  const infoText = createText(
    renderer,
    `\u{1F4CD} ${gitInfo.branch} \u00B7 ${count} files ${label}`,
  );
  headerBox.add(infoText);

  return headerBox;
}

// ── Output box (shared by running + summary) ─────────────────────────────────

function buildOutputBox(state: AppState): BoxRenderable | null {
  if (state.outputLines.length === 0) return null;

  const visible = state.outputLines.slice(-MAX_OUTPUT_LINES);

  const box = new BoxRenderable(state.renderer, {
    border: true,
    borderStyle: "single",
    borderColor: GRAY,
    title: "claude output",
    width: 60,
    marginTop: 1,
    flexDirection: "column",
  });

  for (const line of visible) {
    box.add(createText(state.renderer, line, { attributes: TextAttributes.DIM }));
  }

  return box;
}

// ── Phase 1: Menu ────────────────────────────────────────────────────────────

function showMenu(state: AppState): void {
  state.phase = "menu";
  clearRoot(state);

  const root = state.renderer.root;
  const container = new BoxRenderable(state.renderer, {
    flexDirection: "column",
    padding: 1,
  });
  root.add(container);

  // Header
  container.add(buildHeader(state));

  // Question
  const question = createText(state.renderer, "How would you like to commit?", {
    attributes: TextAttributes.BOLD,
    marginTop: 1,
  });
  container.add(question);

  // Select menu
  const select = new SelectRenderable(state.renderer, {
    options: MENU_OPTIONS,
    selectedIndex: 0,
    showDescription: true,
    wrapSelection: true,
    focusedTextColor: CYAN,
    selectedTextColor: CYAN,
    selectedBackgroundColor: "#1a3a4a",
    marginTop: 1,
  });

  select.on(SelectRenderableEvents.ITEM_SELECTED, (index: number) => {
    const option = MENU_OPTIONS[index];
    if (!option) return;

    if (option.value === "all") {
      git("add", "-A").then(() => {
        startRunning(state, "/commit");
      });
    } else if (option.value === "staged") {
      startRunning(state, "/commit");
    } else if (option.value === "note") {
      showNoteInput(state);
    }
  });

  container.add(select);

  // Focus the select so it receives key events
  state.renderer.focusRenderable(select);
  state.renderer.requestRender();
}

// ── Phase 2: Note Input ──────────────────────────────────────────────────────

function showNoteInput(state: AppState): void {
  state.phase = "note-input";
  state.noteValue = "";
  clearRoot(state);

  const root = state.renderer.root;
  const container = new BoxRenderable(state.renderer, {
    flexDirection: "column",
    padding: 1,
  });
  root.add(container);

  // Header
  container.add(buildHeader(state));

  // Label
  container.add(createText(state.renderer, "Enter commit note:", {
    attributes: TextAttributes.BOLD,
    marginTop: 1,
  }));

  // Input
  const input = new InputRenderable(state.renderer, {
    placeholder: "Describe your changes...",
    marginTop: 1,
  });

  input.on(InputRenderableEvents.ENTER, () => {
    const note = input.value.trim();
    const prompt = note ? `/commit ${note}` : "/commit";
    startRunning(state, prompt);
  });

  container.add(input);

  // Hint
  container.add(createText(state.renderer, "Press enter to submit \u00B7 esc to go back", {
    attributes: TextAttributes.DIM,
  }));

  // Focus input
  state.renderer.focusRenderable(input);

  // Handle esc to go back — listen globally
  const escHandler = (key: KeyEvent) => {
    if (key.name === "escape" && state.phase === "note-input") {
      state.renderer.keyInput.off("keypress", escHandler);
      showMenu(state);
    }
  };
  state.renderer.keyInput.on("keypress", escHandler);

  state.renderer.requestRender();
}

// ── Phase 3: Running ─────────────────────────────────────────────────────────

function startRunning(state: AppState, prompt: string): void {
  state.phase = "running";
  state.outputLines = [];
  state.spinnerFrame = 0;

  renderRunningPhase(state);

  // Start spinner animation
  state.spinnerTimer = setInterval(() => {
    state.spinnerFrame = (state.spinnerFrame + 1) % BRAILLE.length;
    // Re-render running phase to update spinner
    if (state.phase === "running") {
      renderRunningPhase(state);
    }
  }, 80);

  // Spawn subprocess
  runClaude(prompt, {
    onLine: (line: string) => {
      state.outputLines.push(line);
      if (state.phase === "running") {
        renderRunningPhase(state);
      }
    },
    onDone: (result: SubprocessResult) => {
      if (state.spinnerTimer) {
        clearInterval(state.spinnerTimer);
        state.spinnerTimer = null;
      }
      state.summaryResult = result;
      showSummary(state);
    },
  });
}

function renderRunningPhase(state: AppState): void {
  clearRoot(state);

  const root = state.renderer.root;
  const container = new BoxRenderable(state.renderer, {
    flexDirection: "column",
    padding: 1,
  });
  root.add(container);

  // Header
  container.add(buildHeader(state));

  // Spinner line
  const frame = BRAILLE[state.spinnerFrame] ?? BRAILLE[0];
  const spinnerText = createText(
    state.renderer,
    `${frame} Running claude commit...`,
    { fg: CYAN, marginTop: 1 },
  );
  container.add(spinnerText);

  // Output box
  const outputBox = buildOutputBox(state);
  if (outputBox) {
    container.add(outputBox);
  }

  state.renderer.requestRender();
}

// ── Phase 4: Summary ─────────────────────────────────────────────────────────

function showSummary(state: AppState): void {
  state.phase = "summary";
  clearRoot(state);

  const root = state.renderer.root;
  const result = state.summaryResult!;

  const container = new BoxRenderable(state.renderer, {
    flexDirection: "column",
    padding: 1,
  });
  root.add(container);

  // Header
  container.add(buildHeader(state));

  // Output box (full history)
  const outputBox = buildOutputBox(state);
  if (outputBox) {
    container.add(outputBox);
  }

  // Summary box
  if (result.exitCode === 0) {
    const successBox = new BoxRenderable(state.renderer, {
      border: true,
      borderStyle: "rounded",
      borderColor: GREEN,
      title: "\u2705 Committed!",
      width: 55,
      marginTop: 1,
      flexDirection: "column",
    });

    successBox.add(createText(state.renderer, result.logLine, {
      attributes: TextAttributes.BOLD,
    }));
    successBox.add(createText(state.renderer, result.diffStat, {
      attributes: TextAttributes.DIM,
    }));

    container.add(successBox);
  } else {
    const failBox = new BoxRenderable(state.renderer, {
      border: true,
      borderStyle: "rounded",
      borderColor: RED,
      title: "\u274C Failed",
      width: 55,
      marginTop: 1,
      flexDirection: "column",
    });

    failBox.add(createText(
      state.renderer,
      `Claude exited with code ${result.exitCode}`,
    ));

    container.add(failBox);
  }

  // Exit hint
  container.add(createText(state.renderer, "Press q or enter to exit", {
    attributes: TextAttributes.DIM,
    marginTop: 1,
  }));

  // Handle exit keys
  const exitHandler = (key: KeyEvent) => {
    if (key.name === "q" || key.name === "return") {
      state.renderer.keyInput.off("keypress", exitHandler);
      state.renderer.destroy();
      process.exit(result.exitCode);
    }
  };
  state.renderer.keyInput.on("keypress", exitHandler);

  state.renderer.requestRender();
}

// ── Public entry point ───────────────────────────────────────────────────────

export function startApp(renderer: CliRenderer, gitInfo: GitInfo): void {
  const state: AppState = {
    renderer,
    gitInfo,
    phase: "menu",
    outputLines: [],
    summaryResult: null,
    noteValue: "",
    spinnerFrame: 0,
    spinnerTimer: null,
  };

  // Global quit handler (q/esc/ctrl-c in menu)
  renderer.keyInput.on("keypress", (key: KeyEvent) => {
    if (state.phase === "menu") {
      if (key.name === "q" || key.name === "escape") {
        renderer.destroy();
        process.exit(0);
      }
    }
  });

  showMenu(state);
}
