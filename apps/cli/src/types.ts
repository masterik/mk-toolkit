import type { SelectOption } from "@opentui/core";

// ── Phase state (discriminated union) ────────────────────────────────────────

export interface MenuPhase {
  readonly kind: "menu";
}

export interface NoteInputPhase {
  readonly kind: "note-input";
}

export interface RunningPhase {
  readonly kind: "running";
  readonly prompt: string;
}

export interface SummaryPhase {
  readonly kind: "summary";
  readonly exitCode: number;
  readonly logLine: string;
  readonly diffStat: string;
}

export type Phase = MenuPhase | NoteInputPhase | RunningPhase | SummaryPhase;

// ── Git info ─────────────────────────────────────────────────────────────────

export interface GitInfo {
  readonly branch: string;
  readonly stagedCount: number;
  readonly totalCount: number;
}

// ── Menu options ─────────────────────────────────────────────────────────────

export const MENU_OPTIONS: SelectOption[] = [
  { name: "\u{1F680} Commit all", description: "stage everything + commit", value: "all" },
  { name: "\u{1F4E6} Commit staged only", description: "spawn Claude as-is", value: "staged" },
  { name: "\u{1F4DD} Custom note", description: "pass message to Claude", value: "note" },
];
