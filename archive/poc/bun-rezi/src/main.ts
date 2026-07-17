/**
 * POC E — fz commit TUI built with Rezi (@rezi-ui/core + @rezi-ui/node).
 *
 * Run: bun run src/main.ts
 */
import { ui, rgb } from "@rezi-ui/core";
import { createNodeApp } from "@rezi-ui/node";
import { getBranch, getStagedCount, getChangedCount } from "./git.js";
import { spawnClaude } from "./claude.js";
import { type AppState, MENU_ITEMS, SPINNER_FRAMES } from "./types.js";

const t0 = Bun.nanoseconds();

// ── Gather git info ──────────────────────────────────────────────────
const [branch, stagedCount, changedCount] = await Promise.all([
  getBranch(),
  getStagedCount(),
  getChangedCount(),
]);

// ── Create app ───────────────────────────────────────────────────────
const app = createNodeApp<AppState>({
  initialState: {
    phase: "menu",
    branch,
    stagedCount,
    changedCount,
    menuIndex: 0,
    noteText: "",
    spinnerFrame: 0,
    outputLines: [],
    choice: "all",
    exitCode: null,
    commitLine: "",
    diffStat: "",
  },
});

// ── View function ────────────────────────────────────────────────────
app.view((s) => {
  switch (s.phase) {
    case "menu":
      return menuView(s);
    case "note":
      return noteView(s);
    case "running":
      return runningView(s);
    case "summary":
      return summaryView(s);
  }
});

// ── Phase 1: Menu ────────────────────────────────────────────────────
function menuView(s: Readonly<AppState>) {
  const menuChildren = MENU_ITEMS.map((item, i) => {
    const selected = i === s.menuIndex;
    const prefix = selected ? "\u25B6" : " ";
    const style = selected
      ? { bold: true, inverse: true }
      : {};
    return ui.text(`${prefix} ${item.label}  ${item.desc}`, style);
  });

  return ui.page({
    p: 1,
    gap: 1,
    header: ui.header({ title: `\u26A1 ForgeZ \u2014 commit` }),
    body: ui.column({ gap: 1 }, [
      ui.box(
        { border: "rounded", title: "info", p: 1 },
        [
          ui.text(`Branch: ${s.branch}`),
          ui.row({ gap: 2 }, [
            ui.text(`Staged: ${s.stagedCount}`),
            ui.text(`Changed: ${s.changedCount}`),
          ]),
        ],
      ),
      ui.text("How would you like to commit?"),
      ui.column({ gap: 0 }, menuChildren),
      ui.text("Enter to select \u00B7 q/esc to quit", { dim: true }),
    ]),
  });
}

// ── Phase 2: Note input ──────────────────────────────────────────────
function noteView(s: Readonly<AppState>) {
  return ui.page({
    p: 1,
    gap: 1,
    header: ui.header({ title: `\u26A1 ForgeZ \u2014 commit` }),
    body: ui.column({ gap: 1 }, [
      ui.text("Enter commit note:"),
      ui.input({
        id: "note-input",
        value: s.noteText,
        onInput: (value) => {
          app.update((prev) => ({ ...prev, noteText: value }));
        },
      }),
      ui.text("Press enter to submit \u00B7 esc to go back", { dim: true }),
    ]),
  });
}

// ── Phase 3: Running ─────────────────────────────────────────────────
function runningView(s: Readonly<AppState>) {
  const frame = SPINNER_FRAMES[s.spinnerFrame];
  const outputText =
    s.outputLines.length > 0 ? s.outputLines.join("\n") : "(waiting for output...)";

  return ui.page({
    p: 1,
    gap: 1,
    header: ui.header({ title: `\u26A1 ForgeZ \u2014 commit` }),
    body: ui.column({ gap: 1 }, [
      ui.text(`${frame} Running claude commit...`, { bold: true }),
      ui.box(
        { border: "rounded", title: "claude output", p: 1 },
        [ui.text(outputText)],
      ),
    ]),
  });
}

// ── Phase 4: Summary ─────────────────────────────────────────────────
function summaryView(s: Readonly<AppState>) {
  const success = s.exitCode === 0;
  const borderColor = success ? rgb(0, 200, 0) : rgb(200, 0, 0);
  const title = success ? "\u2705 Committed!" : "\u274C Failed";

  const summaryContent = success
    ? [
        ui.text(s.commitLine),
        s.diffStat ? ui.text(s.diffStat, { dim: true }) : null,
      ].filter(Boolean) as ReturnType<typeof ui.text>[]
    : [ui.text(`Exit code: ${s.exitCode}`, { fg: rgb(200, 0, 0) })];

  const outputText =
    s.outputLines.length > 0 ? s.outputLines.join("\n") : "(no output)";

  return ui.page({
    p: 1,
    gap: 1,
    header: ui.header({ title: `\u26A1 ForgeZ \u2014 commit` }),
    body: ui.column({ gap: 1 }, [
      ui.box(
        {
          border: "rounded",
          title,
          style: { fg: borderColor },
          p: 1,
        },
        summaryContent,
      ),
      ui.box(
        { border: "rounded", title: "claude output", p: 1 },
        [ui.text(outputText)],
      ),
      ui.text("Press q or enter to exit", { dim: true }),
    ]),
  });
}

// ── Key bindings ─────────────────────────────────────────────────────
app.keys({
  q: ({ state }) => {
    if (state.phase === "menu" || state.phase === "summary") {
      app.stop();
    }
  },
  escape: ({ state, update }) => {
    if (state.phase === "menu") {
      app.stop();
    } else if (state.phase === "note") {
      update((prev) => ({ ...prev, phase: "menu" }));
    }
  },
  "ctrl+c": () => {
    app.stop();
  },
  up: ({ state, update }) => {
    if (state.phase === "menu") {
      update((prev) => ({
        ...prev,
        menuIndex:
          prev.menuIndex <= 0 ? MENU_ITEMS.length - 1 : prev.menuIndex - 1,
      }));
    }
  },
  down: ({ state, update }) => {
    if (state.phase === "menu") {
      update((prev) => ({
        ...prev,
        menuIndex:
          prev.menuIndex >= MENU_ITEMS.length - 1 ? 0 : prev.menuIndex + 1,
      }));
    }
  },
  k: ({ state, update }) => {
    if (state.phase === "menu") {
      update((prev) => ({
        ...prev,
        menuIndex:
          prev.menuIndex <= 0 ? MENU_ITEMS.length - 1 : prev.menuIndex - 1,
      }));
    }
  },
  j: ({ state, update }) => {
    if (state.phase === "menu") {
      update((prev) => ({
        ...prev,
        menuIndex:
          prev.menuIndex >= MENU_ITEMS.length - 1 ? 0 : prev.menuIndex + 1,
      }));
    }
  },
  enter: ({ state, update }) => {
    if (state.phase === "menu") {
      const item = MENU_ITEMS[state.menuIndex];
      if (item.choice === "note") {
        update((prev) => ({ ...prev, phase: "note", choice: "note" }));
      } else {
        update((prev) => ({
          ...prev,
          phase: "running",
          choice: item.choice,
        }));
        // Defer spawn to next tick so state is committed
        setTimeout(() => spawnClaude(app, item.choice, ""), 0);
      }
    } else if (state.phase === "note") {
      const note = state.noteText;
      update((prev) => ({ ...prev, phase: "running" }));
      setTimeout(() => spawnClaude(app, "note", note), 0);
    } else if (state.phase === "summary") {
      app.stop();
    }
  },
});

// ── Start ────────────────────────────────────────────────────────────
const readyMs = (Bun.nanoseconds() - t0) / 1_000_000;
console.error(`[fz] ready in ${readyMs.toFixed(1)}ms`);

await app.run();
