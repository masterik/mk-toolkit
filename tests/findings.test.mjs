// node --test tests/findings.test.mjs
//
// scripts/findings.mjs is invoked as a subprocess (it is a CLI, not a module with
// exports) so these tests exercise the same interface the skills call: argv in,
// stdout/exit-code/reconciled.jsonl out.

import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const SCRIPT = new URL("../scripts/findings.mjs", import.meta.url).pathname;

function run(args, { expectFail = false } = {}) {
  try {
    const out = execFileSync("node", [SCRIPT, ...args], { encoding: "utf8" });
    if (expectFail) assert.fail(`expected failure, got:\n${out}`);
    return { stdout: out, code: 0 };
  } catch (e) {
    if (!expectFail) assert.fail(`unexpected failure (${e.status}):\n${e.stdout}${e.stderr}`);
    return { stdout: e.stdout ?? "", stderr: e.stderr ?? "", code: e.status };
  }
}

function withRunDir(fn) {
  const dir = mkdtempSync(join(tmpdir(), "mkit-findings-"));
  try {
    return fn(dir);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const jsonl = (records) => records.map((r) => JSON.stringify(r)).join("\n") + "\n";
const readJsonl = (path) =>
  readFileSync(path, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((l) => JSON.parse(l));

const FINDING = (over = {}) => ({
  surface: "code",
  severity: "major",
  file: "src/a.ts",
  title: "missing null check",
  body: "crashes on empty input",
  confidence: 70,
  line: 10,
  ...over,
});

test("schema prints without touching a run dir", () => {
  const { stdout } = run(["schema"]);
  assert.match(stdout, /findings-<source>\.jsonl/);
});

test("validate reports missing required fields", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "findings-a.jsonl"), jsonl([{ severity: "major" }]));
    const { stdout, code } = run(["validate", dir], { expectFail: true });
    assert.equal(code, 1);
    assert.match(stdout, /missing surface/);
    assert.match(stdout, /missing file/);
    assert.match(stdout, /missing title/);
  });
});

test("validate rejects an out-of-range confidence and a non-integer line", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-a.jsonl"),
      jsonl([FINDING({ confidence: 150, line: 1.5 })]),
    );
    const { stdout, code } = run(["validate", dir], { expectFail: true });
    assert.equal(code, 1);
    assert.match(stdout, /confidence must be 0-100/);
    assert.match(stdout, /line must be an integer/);
  });
});

test("validate passes a well-formed file", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "findings-a.jsonl"), jsonl([FINDING()]));
    const { stdout, code } = run(["validate", dir]);
    assert.equal(code, 0);
    assert.match(stdout, /\bok\b/);
  });
});

test("reconcile merges same-file near-line findings from independent sources into one, with a confidence boost", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-codex.jsonl"),
      jsonl([FINDING({ title: "missing null check", line: 10, confidence: 70 })]),
    );
    writeFileSync(
      join(dir, "findings-claude.jsonl"),
      jsonl([FINDING({ title: "npe on empty input", line: 11, confidence: 60 })]),
    );
    const { stdout } = run(["reconcile", dir, "--sources-expected", "2"]);
    assert.match(stdout, /findings=1/);
    assert.match(stdout, /merged=1/);
    const reconciled = readJsonl(join(dir, "reconciled.jsonl"));
    assert.equal(reconciled.length, 1);
    assert.equal(reconciled[0].confidence, 80); // 70 base + 10 for a second source
    assert.deepEqual(reconciled[0].sources.sort(), ["claude", "codex"]);
  });
});

test("reconcile does not merge findings more than the line window apart", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "findings-a.jsonl"), jsonl([FINDING({ line: 10 })]));
    writeFileSync(join(dir, "findings-b.jsonl"), jsonl([FINDING({ line: 50 })]));
    const { stdout } = run(["reconcile", dir, "--sources-expected", "2"]);
    assert.match(stdout, /findings=2/);
    assert.match(stdout, /merged=0/);
  });
});

test("reconcile normalizes absolute vs repo-relative paths before comparing", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-a.jsonl"),
      jsonl([FINDING({ file: "src/a.ts", line: 10 })]),
    );
    writeFileSync(
      join(dir, "findings-b.jsonl"),
      jsonl([FINDING({ file: "/repo/src/a.ts", line: 10 })]),
    );
    const { stdout } = run(["reconcile", dir, "--sources-expected", "2"]);
    assert.match(stdout, /findings=1/);
  });
});

test("reconcile drops a weak single-source minor finding only when every source reported", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-a.jsonl"),
      jsonl([FINDING({ severity: "minor", confidence: 50 })]),
    );
    writeFileSync(join(dir, "findings-b.jsonl"), jsonl([]));
    const { stdout } = run(["reconcile", dir, "--sources-expected", "2"]);
    assert.match(stdout, /complete=true/);
    assert.match(stdout, /dropped=1/);
    assert.equal(readJsonl(join(dir, "reconciled.jsonl")).length, 0);
  });
});

test("reconcile disables the drop rule when a source is missing", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-a.jsonl"),
      jsonl([FINDING({ severity: "minor", confidence: 50 })]),
    );
    const { stdout } = run(["reconcile", dir, "--sources-expected", "2"]);
    assert.match(stdout, /complete=false/);
    assert.match(stdout, /dropped=0/);
    assert.match(stdout, /drop_rule=disabled/);
    assert.equal(readJsonl(join(dir, "reconciled.jsonl")).length, 1);
  });
});

test("reconcile keeps open_question/pre_existing records aside, not merged as findings", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "findings-a.jsonl"),
      jsonl([FINDING(), { ...FINDING(), class: "open_question", title: "is this intended?" }]),
    );
    const { stdout } = run(["reconcile", dir, "--sources-expected", "1"]);
    assert.match(stdout, /findings=1/);
    assert.match(stdout, /aside=1/);
    const ids = readJsonl(join(dir, "reconciled.jsonl")).map((r) => r.id);
    assert.deepEqual(ids.sort(), ["f01", "x01"]);
  });
});

test("reconcile fails loudly with no findings-*.jsonl present", () => {
  withRunDir((dir) => {
    const { code } = run(["reconcile", dir], { expectFail: true });
    assert.equal(code, 1);
  });
});

test("reconcile rejects a --sources-expected with no value", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "findings-a.jsonl"), jsonl([FINDING()]));
    const { stderr, code } = run(["reconcile", dir, "--sources-expected"], { expectFail: true });
    assert.equal(code, 2);
    assert.match(stderr, /needs a numeric value/);
  });
});

test("group splits findings by directory and writes one verify file per group", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "reconciled.jsonl"),
      jsonl([
        { ...FINDING(), id: "f01", file: "src/a.ts" },
        { ...FINDING(), id: "f02", file: "docs/readme.md" },
      ]),
    );
    // Folding small groups together (the default min-per-group=3) would merge these
    // two single-finding directories back into one — turn that off to see the raw split.
    const { stdout } = run(["group", dir, "--min-per-group", "1"]);
    assert.match(stdout, /groups=2/);
    const g1 = readJsonl(join(dir, "verify-g1.jsonl"));
    const g2 = readJsonl(join(dir, "verify-g2.jsonl"));
    assert.equal(g1.length + g2.length, 2);
  });
});

test("group folds directories smaller than min-per-group together by default", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "reconciled.jsonl"),
      jsonl([
        { ...FINDING(), id: "f01", file: "src/a.ts" },
        { ...FINDING(), id: "f02", file: "docs/readme.md" },
      ]),
    );
    const { stdout } = run(["group", dir]);
    assert.match(stdout, /groups=1/);
  });
});

test("group with a handful of findings suggests inline verification", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "reconciled.jsonl"), jsonl([{ ...FINDING(), id: "f01" }]));
    const { stdout } = run(["group", dir]);
    assert.match(stdout, /suggest=inline/);
  });
});

test("group fails without a prior reconcile", () => {
  withRunDir((dir) => {
    const { code } = run(["group", dir], { expectFail: true });
    assert.equal(code, 1);
  });
});

test("report merges verdicts onto findings and flags unverified ones", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "reconciled.jsonl"),
      jsonl([
        { ...FINDING(), id: "f01" },
        { ...FINDING(), id: "f02", title: "second" },
      ]),
    );
    writeFileSync(
      join(dir, "verdicts-g1.jsonl"),
      jsonl([{ id: "f01", verdict: "confirmed", reason: "reproduced" }]),
    );
    const { stdout } = run(["report", dir]);
    assert.match(stdout, /reportable=1/);
    assert.match(stdout, /UNVERIFIED=f02/);
    const final = readJsonl(join(dir, "final.jsonl"));
    assert.equal(final.find((r) => r.id === "f01").verdict, "confirmed");
    assert.equal(final.find((r) => r.id === "f02").verdict, null);
  });
});

test("report applies a refined verdict's field corrections", () => {
  withRunDir((dir) => {
    writeFileSync(
      join(dir, "reconciled.jsonl"),
      jsonl([{ ...FINDING(), id: "f01", severity: "minor" }]),
    );
    writeFileSync(
      join(dir, "verdicts-g1.jsonl"),
      jsonl([{ id: "f01", verdict: "refined", severity: "critical", reason: "worse than reported" }]),
    );
    run(["report", dir]);
    const final = readJsonl(join(dir, "final.jsonl"));
    assert.equal(final[0].severity, "critical");
    assert.equal(final[0].verdict, "refined");
  });
});

test("report treats an unrecognized verdict as an orphan, not applied", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "reconciled.jsonl"), jsonl([{ ...FINDING(), id: "f01" }]));
    writeFileSync(join(dir, "verdicts-g1.jsonl"), jsonl([{ id: "f01", verdict: "maybe" }]));
    const { stdout } = run(["report", dir]);
    assert.match(stdout, /unknown verdict maybe/);
    assert.match(stdout, /UNVERIFIED=f01/);
  });
});

test("report flags a verdict for an id that does not exist as an orphan", () => {
  withRunDir((dir) => {
    writeFileSync(join(dir, "reconciled.jsonl"), jsonl([{ ...FINDING(), id: "f01" }]));
    writeFileSync(join(dir, "verdicts-g1.jsonl"), jsonl([{ id: "f99", verdict: "confirmed" }]));
    const { stdout } = run(["report", dir]);
    assert.match(stdout, /verdict for unknown id f99/);
  });
});

test("bad usage exits 2", () => {
  const { code } = run([], { expectFail: true });
  assert.equal(code, 2);
});
