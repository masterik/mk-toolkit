/** Phase discriminator for the fz commit TUI. */
export type Phase = "menu" | "note" | "running" | "summary";

/** Menu item choices. */
export type MenuChoice = "all" | "staged" | "note";

/** Application state. */
export interface AppState {
  phase: Phase;

  // Git info
  branch: string;
  stagedCount: number;
  changedCount: number;

  // Menu
  menuIndex: number;

  // Note input
  noteText: string;

  // Running phase
  spinnerFrame: number;
  outputLines: string[];
  choice: MenuChoice;

  // Summary phase
  exitCode: number | null;
  commitLine: string;
  diffStat: string;
}

export const MENU_ITEMS: readonly {
  label: string;
  desc: string;
  choice: MenuChoice;
}[] = [
  { label: "\u{1F680} Commit all", desc: "stage everything + commit", choice: "all" },
  { label: "\u{1F4E6} Commit staged only", desc: "spawn Claude as-is", choice: "staged" },
  { label: "\u{1F4DD} Custom note", desc: "pass message to Claude", choice: "note" },
];

export const SPINNER_FRAMES = [
  "\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F",
];
