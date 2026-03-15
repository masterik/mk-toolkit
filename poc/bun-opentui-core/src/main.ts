#!/usr/bin/env bun
/**
 * ForgeZ POC D — fz commit (Bun + @opentui/core, imperative API, NO React)
 *
 * Exercises: styled boxes, interactive menu, spinner, subprocess streaming, summary display.
 * All rendering is done via direct Renderable class instantiation — no JSX, no React.
 *
 * Run: bun run src/main.ts
 */

const startNs = Bun.nanoseconds();

import { createCliRenderer } from "@opentui/core";
import { fetchGitInfo } from "./git.ts";
import { startApp } from "./ui.ts";

async function main() {
  const readyMs = ((Bun.nanoseconds() - startNs) / 1_000_000).toFixed(0);
  process.stderr.write(`\x1b[2m[fz] ready in ${readyMs}ms\x1b[0m\n`);

  const gitInfo = await fetchGitInfo();

  const renderer = await createCliRenderer({ exitOnCtrlC: true });
  startApp(renderer, gitInfo);
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
