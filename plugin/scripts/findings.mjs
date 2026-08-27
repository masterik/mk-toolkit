#!/usr/bin/env node
//
// Mechanical work on a review run's findings: validate, reconcile, group, report.
//
//   node findings.mjs schema
//   node findings.mjs validate  <run-dir>
//   node findings.mjs reconcile <run-dir> --sources-expected 3 [--sim 0.6] [--band 0.3]
//   node findings.mjs group     <run-dir> [--max-groups 8]
//   node findings.mjs report    <run-dir>
//
// Why Node and not shell: this is the one stage that is data, not process. ±2-line
// windowing, set arithmetic over sources, a confidence formula, stable ids and a
// directory grouping are all one-liners over real objects and write-only code as jq
// pipelines. Startup is ~23 ms — about one `git status`.
//
// What it does NOT do, on purpose: it never decides whether two findings are the same
// *problem* (it merges only near-identical text and hands back a review list for the
// rest), never assigns severity, never judges materiality, and never edits a file.
// Those are the reviewer's and the skill's calls.
//
// Exit: 0 ok, 1 malformed input, 2 bad usage.

import { readdirSync, readFileSync, writeFileSync, existsSync, statSync } from "node:fs";
import { join, dirname, basename } from "node:path";

const SEVERITIES = ["minor", "major", "critical"];
const SURFACES = ["code", "comments", "docs", "tests", "config", "build"];
const VERDICTS = ["confirmed", "refined", "rejected", "immaterial", "pre_existing"];
const CLASSES = ["finding", "open_question", "pre_existing"];

const die = (msg, code = 2) => {
  process.stderr.write(`findings: ${msg}\n`);
  process.exit(code);
};

// ---------------------------------------------------------------- io

function readJsonl(path) {
  const out = [];
  const errors = [];
  const text = readFileSync(path, "utf8");
  text.split("\n").forEach((raw, i) => {
    const line = raw.trim();
    if (!line || line.startsWith("//")) return;
    try {
      out.push(JSON.parse(line));
    } catch (e) {
      errors.push(`${basename(path)}:${i + 1}: ${e.message}`);
    }
  });
  return { records: out, errors };
}

const writeJsonl = (path, records) =>
  writeFileSync(path, records.map((r) => JSON.stringify(r)).join("\n") + "\n");

function runFiles(runDir, prefix) {
  if (!existsSync(runDir) || !statSync(runDir).isDirectory())
    die(`run directory does not exist: ${runDir}`);
  return readdirSync(runDir)
    .filter((f) => f.startsWith(prefix) && f.endsWith(".jsonl"))
    .sort()
    .map((f) => join(runDir, f));
}

// ---------------------------------------------------------------- validation

function validate(rec, where) {
  // A JSONL line of `null`, `42` or `[]` parses fine; indexing it must report, not throw.
  if (rec === null || typeof rec !== "object" || Array.isArray(rec))
    return [`${where}: not a JSON object`];
  const problems = [];
  const need = (k) => {
    if (rec[k] === undefined || rec[k] === null || rec[k] === "") problems.push(`missing ${k}`);
  };
  ["surface", "severity", "file", "title"].forEach(need);
  if (rec.severity && !SEVERITIES.includes(rec.severity))
    problems.push(`severity not one of ${SEVERITIES.join("|")}: ${rec.severity}`);
  if (rec.surface && !SURFACES.includes(rec.surface))
    problems.push(`surface not one of ${SURFACES.join("|")}: ${rec.surface}`);
  if (rec.class && !CLASSES.includes(rec.class)) problems.push(`class unknown: ${rec.class}`);
  const c = rec.confidence;
  if (c !== undefined && (typeof c !== "number" || c < 0 || c > 100))
    problems.push(`confidence must be 0-100: ${c}`);
  if (rec.line !== undefined && rec.line !== null && !Number.isInteger(rec.line))
    problems.push(`line must be an integer: ${rec.line}`);
  return problems.map((p) => `${where}: ${p}`);
}

// ---------------------------------------------------------------- similarity

const STOP = new Set(
  ("a an the is are was were be been being this that these those it its of to in on for and or " +
    "not no if then than when where which with without into from at by as but can could should " +
    "would may might will shall do does did done has have had here there we you i").split(" "),
);

const tokens = (s) =>
  new Set(
    String(s || "")
      .toLowerCase()
      .replace(/[^a-z0-9_]+/g, " ")
      .split(" ")
      .filter((t) => t.length > 2 && !STOP.has(t)),
  );

function jaccard(a, b) {
  if (!a.size || !b.size) return 0;
  let shared = 0;
  for (const t of a) if (b.has(t)) shared++;
  return shared / (a.size + b.size - shared);
}

// ---------------------------------------------------------------- reconcile

// Three independent tools write these files, and absolute-vs-repo-relative is exactly where
// they diverge. A raw `!==` left the same defect from two sources as two findings, costing the
// merge and the +10 corroboration boost — the whole point of this stage. Compare on a
// normalized tail: separators unified, leading ./ and / dropped.
const normPath = (p) =>
  String(p ?? "")
    .replace(/\\/g, "/")
    .replace(/^\.\//, "")
    .replace(/^\/+/, "");
const samePath = (a, b) => {
  const x = normPath(a);
  const y = normPath(b);
  if (x === y) return true;
  // One may be absolute while the other is repo-relative: accept a full path-segment suffix.
  const shorter = x.length <= y.length ? x : y;
  const longer = x.length <= y.length ? y : x;
  return shorter !== "" && longer.endsWith(`/${shorter}`);
};

const sevRank = (s) => Math.max(0, SEVERITIES.indexOf(s));
const maxSev = (a, b) => (sevRank(a) >= sevRank(b) ? a : b);
const sourcesOf = (r) => (Array.isArray(r.sources) ? r.sources : [r.source].filter(Boolean));
const lensesOf = (r) => {
  const l = r.lenses ?? r.lens;
  return Array.isArray(l) ? l : l ? [l] : [];
};

function reconcile(runDir, opts) {
  const files = runFiles(runDir, "findings-");
  if (!files.length) die(`no findings-*.jsonl in ${runDir}`, 1);

  const all = [];
  const errors = [];
  const sourcesSeen = new Set();
  for (const f of files) {
    const { records, errors: errs } = readJsonl(f);
    errors.push(...errs);
    const fallbackSource = basename(f).replace(/^findings-|\.jsonl$/g, "");
    // A present file means that source reported, even with zero findings: absent and
    // present-but-empty must not collapse, or the drop rule re-arms on a missing source.
    sourcesSeen.add(fallbackSource);
    records.forEach((r, i) => {
      if (r === null || typeof r !== "object" || Array.isArray(r)) {
        errors.push(`${basename(f)}#${i + 1}: not a JSON object`);
        return;
      }
      r.source ||= fallbackSource;
      sourcesSeen.add(r.source);
      errors.push(...validate(r, `${basename(f)}#${i + 1}`));
      all.push(r);
    });
  }
  if (errors.length) {
    process.stdout.write(`invalid=${errors.length}\n` + errors.slice(0, 20).join("\n") + "\n");
    if (errors.length > 20) process.stdout.write(`... ${errors.length - 20} more\n`);
    process.exit(1);
  }

  // 1. Split out what is not a defect in the change, before anything is merged.
  const aside = all.filter((r) => r.class && r.class !== "finding");
  const defects = all.filter((r) => !r.class || r.class === "finding");

  // 2. Merge on the documented key: same file, within ±window lines. The text does not
  //    gate the merge — measured on real reviewer output, two reviewers describing one
  //    missing `await` at lines 42 and 43 share 0.23 of their tokens, so any similarity
  //    gate safe enough to trust would let every real duplicate through. Location decides.
  //
  //    Nothing is discarded by merging: every member's title and body is kept on the
  //    survivor (`also`), so a merge that turns out to be two problems is still visible
  //    to the verifier rather than lost. Low-similarity merges are flagged for a look.
  const clusters = [];
  const review = [];
  const flags = [];
  const clusterOf = new Map();
  for (const r of defects) {
    let placed = false;
    for (const c of clusters) {
      if (!samePath(c.head.file, r.file)) continue;
      // Integers only: `line: null` is permitted by validate, and null !== undefined would
      // make Math.abs(null - null) === 0 merge two unrelated findings in the same file.
      const bothLocated = Number.isInteger(c.head.line) && Number.isInteger(r.line);
      const near = bothLocated && Math.abs(c.head.line - r.line) <= opts.window;
      const sim = Number(
        jaccard(tokens(`${c.head.title} ${c.head.body}`), tokens(`${r.title} ${r.body}`)).toFixed(2),
      );
      if (near) {
        c.members.push(r);
        clusterOf.set(r, c);
        // Accumulate: in a 3+ member cluster, assigning would keep only the last flag.
        if (sim < opts.band) (c.lowSim ||= []).push({ sim, other: r.title });
        placed = true;
        break;
      }
      if (sim >= opts.sim) review.push({ a: c, b: r, sim });
    }
    if (!placed) {
      const c = { head: r, members: [r] };
      clusters.push(c);
      clusterOf.set(r, c);
    }
  }

  // 3-5. Confidence from distinct sources, severity from the highest claim, and the
  //      weak-singleton drop — disabled outright when a source is missing.
  const complete = sourcesSeen.size >= opts.sourcesExpected;
  const survivors = [];
  const dropped = [];
  for (const c of clusters) {
    const srcs = [...new Set(c.members.flatMap(sourcesOf))];
    const lenses = [...new Set(c.members.flatMap(lensesOf))];
    const bestConf = Math.max(...c.members.map((m) => m.confidence ?? 50));
    const severity = c.members.map((m) => m.severity).reduce(maxSev, "minor");
    const confidence = Math.min(99, bestConf + 10 * (srcs.length - 1));
    // Primary = the member that claims the most; the rest ride along in `also`.
    const ordered = [...c.members].sort(
      (x, y) => sevRank(y.severity) - sevRank(x.severity) || (y.confidence ?? 0) - (x.confidence ?? 0),
    );
    const also = ordered
      .slice(1)
      .map((m) => ({ source: m.source, title: m.title, body: m.body, line: m.line }));
    const merged = {
      ...ordered[0],
      ...(also.length ? { also } : {}),
      sources: srcs,
      lenses,
      severity,
      confidence,
      merged_count: c.members.length,
      class: "finding",
    };
    delete merged.source;
    delete merged.lens;

    const weak = srcs.length === 1 && confidence < 80 && sevRank(severity) === 0;
    if (weak && complete) {
      dropped.push({ ...merged, drop_reason: `single source ${srcs[0]}, conf ${confidence}, minor` });
    } else {
      c.out = merged;
      survivors.push(merged);
    }
  }

  // Stable ids: severity, then confidence, then location. Same input, same ids.
  survivors.sort(
    (a, b) =>
      sevRank(b.severity) - sevRank(a.severity) ||
      (b.confidence ?? 0) - (a.confidence ?? 0) ||
      String(a.file).localeCompare(String(b.file)) ||
      (a.line ?? 0) - (b.line ?? 0),
  );
  survivors.forEach((r, i) => (r.id = `f${String(i + 1).padStart(2, "0")}`));
  aside.forEach((r, i) => (r.id = `x${String(i + 1).padStart(2, "0")}`));

  const out = join(runDir, "reconciled.jsonl");
  writeJsonl(out, [...survivors, ...aside]);

  const tally = (rs) => {
    const m = new Map();
    for (const r of rs) {
      const k = `[${r.surface}, ${r.severity}]`;
      m.set(k, (m.get(k) ?? 0) + 1);
    }
    return [...m].sort().map(([k, v]) => `${v} ${k}`).join(", ") || "none";
  };

  const L = [];
  L.push(`wrote=${out}`);
  L.push(`sources_present=${[...sourcesSeen].sort().join(",")} expected=${opts.sourcesExpected} complete=${complete}`);
  L.push(`in=${all.length} findings=${survivors.length} merged=${defects.length - clusters.length} dropped=${dropped.length} aside=${aside.length}`);
  L.push(`counts=${tally(survivors)}`);
  if (!complete) L.push("drop_rule=disabled (a source is missing or degraded)");
  for (const c of clusters) {
    if (c.out && c.members.length > 1)
      L.push(
        `merged ${c.out.id} ${c.out.file}:${c.members.map((m) => m.line ?? "?").join("~")} ` +
          `[${c.out.sources.join("+")}]${
            c.lowSim?.length
              ? ` LOW-SIM ${c.lowSim.map((f) => f.sim).join(",")} — also titled ` +
                c.lowSim.map((f) => `"${f.other}"`).join(" and ") +
                "; check it is one problem"
              : ""
          }`,
      );
  }
  for (const d of dropped) L.push(`dropped ${d.file}:${d.line ?? "?"} — ${d.title} — ${d.drop_reason}`);
  for (const r of aside) L.push(`${r.id} ${r.class} ${r.file}:${r.line ?? "?"} — ${r.title}`);
  if (review.length) {
    L.push(`review_pairs=${review.length} — same file, similar wording, different lines: one shape at two sites, or two findings? your call`);
    for (const p of review.slice(0, 10))
      L.push(
        `  sim=${p.sim} ${p.a.out?.id ?? "?"} ${p.a.head.file}:${p.a.head.line ?? "?"} "${p.a.head.title}"` +
          ` <-> ${clusterOf.get(p.b)?.out?.id ?? "dropped"} :${p.b.line ?? "?"} "${p.b.title}"`,
      );
    if (review.length > 10) L.push(`  ... ${review.length - 10} more`);
  }
  process.stdout.write(L.join("\n") + "\n");
}

// ---------------------------------------------------------------- group

function group(runDir, opts) {
  const file = join(runDir, "reconciled.jsonl");
  if (!existsSync(file)) die(`no reconciled.jsonl in ${runDir} — run reconcile first`, 1);
  const { records } = readJsonl(file);
  const defects = records.filter((r) => !r.class || r.class === "finding");

  const byDir = new Map();
  for (const r of defects) {
    const d = dirname(String(r.file)) || ".";
    if (!byDir.has(d)) byDir.set(d, []);
    byDir.get(d).push(r);
  }
  // Fold the smallest directories together until every group is worth a round trip and
  // there are no more than max-groups of them. One verifier per *finding* is the failure
  // mode this avoids: the subagent round trip costs more than the verification.
  let groups = [...byDir.entries()].map(([dir, rs]) => ({ name: dir, records: rs }));
  groups.sort((a, b) => b.records.length - a.records.length);
  while (
    groups.length > 1 &&
    (groups.length > opts.maxGroups || groups[groups.length - 1].records.length < opts.minPer)
  ) {
    const small = groups.pop();
    const target = groups[groups.length - 1];
    target.records.push(...small.records);
    target.name = `${target.name}+`;
    groups.sort((a, b) => b.records.length - a.records.length);
  }

  const L = [];
  groups.forEach((g, i) => {
    const slug = `g${i + 1}`;
    const out = join(runDir, `verify-${slug}.jsonl`);
    writeJsonl(out, g.records);
    L.push(`${slug} ${g.name} n=${g.records.length} ids=${g.records.map((r) => r.id).join(",")} file=${out}`);
  });
  const files = new Set(defects.map((r) => r.file)).size;
  L.push(`groups=${groups.length} findings=${defects.length} files=${files}`);
  // The documented threshold, applied — still a suggestion: the skill decides.
  L.push(
    defects.length <= 5 && groups.length === 1
      ? "suggest=inline (a handful of findings, one group: verify here, write verdicts-all.jsonl — a lone subagent buys independence you already have)"
      : "suggest=fanout (one verifier per group, spawned in one message)",
  );
  process.stdout.write(L.join("\n") + "\n");
}

// ---------------------------------------------------------------- report

function report(runDir) {
  const rec = join(runDir, "reconciled.jsonl");
  if (!existsSync(rec)) die(`no reconciled.jsonl in ${runDir}`, 1);
  const findings = new Map();
  for (const r of readJsonl(rec).records) findings.set(r.id, r);

  const verdicts = new Map();
  const orphans = [];
  const vfiles = runFiles(runDir, "verdicts-");
  for (const f of vfiles) {
    for (const v of readJsonl(f).records) {
      if (!v.id || !findings.has(v.id)) {
        orphans.push(`${basename(f)}: verdict for unknown id ${v.id ?? "(none)"}`);
        continue;
      }
      if (!v.verdict || !VERDICTS.includes(v.verdict)) {
        // Do not store it: an unrecognised verdict that lands on the finding satisfies the
        // UNVERIFIED check while meaning nothing. Leave the id unverified and say so.
        orphans.push(`${basename(f)}: ${v.id} unknown verdict ${v.verdict ?? "(none)"} — not applied`);
        continue;
      }
      verdicts.set(v.id, { ...verdicts.get(v.id), ...v });
    }
  }

  const merged = [...findings.values()].map((f) => {
    const v = verdicts.get(f.id);
    // A `refined` verdict may correct fields; omitted fields keep their original.
    const { id, verdict, reason, ...corrections } = v ?? {};
    return { ...f, ...(verdict === "refined" ? corrections : {}), verdict: verdict ?? null, verdict_reason: reason ?? null };
  });
  writeJsonl(join(runDir, "final.jsonl"), merged);

  const defects = merged.filter((r) => !r.class || r.class === "finding");
  const byVerdict = new Map();
  for (const r of defects) byVerdict.set(r.verdict ?? "no-verdict", (byVerdict.get(r.verdict ?? "no-verdict") ?? 0) + 1);

  const kept = defects.filter((r) => r.verdict === "confirmed" || r.verdict === "refined");
  const tag = (r) => `[${r.surface}, ${r.severity}]`;
  const counts = new Map();
  for (const r of kept) counts.set(tag(r), (counts.get(tag(r)) ?? 0) + 1);

  const L = [];
  L.push(`wrote=${join(runDir, "final.jsonl")}`);
  L.push(`verdict_files=${vfiles.length} ${vfiles.map((f) => basename(f)).join(",") || "none"}`);
  L.push(`verdicts=${[...byVerdict].map(([k, v]) => `${k}:${v}`).join(" ")}`);
  L.push(`reportable=${kept.length} counts=${[...counts].sort().map(([k, v]) => `${v} ${k}`).join(", ") || "none"}`);
  L.push(`gating=${kept.filter((r) => sevRank(r.severity) > 0).length} (critical+major confirmed)`);
  const missing = defects.filter((r) => !r.verdict).map((r) => r.id);
  if (missing.length) L.push(`UNVERIFIED=${missing.join(",")} — a lost verdict reads exactly like a finding nobody raised`);
  for (const o of orphans.slice(0, 10)) L.push(`orphan ${o}`);
  for (const r of kept) L.push(`${r.id} ${tag(r)} conf ${r.confidence} ${r.verdict} ${r.file}:${r.line ?? "?"} — ${r.title}${r.sources?.length > 1 ? ` [${r.sources.join("+")}]` : ""}`);
  for (const r of defects.filter((r) => r.verdict && !kept.includes(r)))
    L.push(`${r.id} ${r.verdict} ${r.file}:${r.line ?? "?"} — ${r.title}${r.verdict_reason ? ` — ${r.verdict_reason}` : ""}`);
  for (const r of merged.filter((r) => r.class && r.class !== "finding"))
    L.push(`${r.id} ${r.class} ${r.file}:${r.line ?? "?"} — ${r.title}`);
  process.stdout.write(L.join("\n") + "\n");
}

// ---------------------------------------------------------------- cli

const SCHEMA = `findings-<source>.jsonl — one JSON object per line, written by each reviewer:
{"surface":"code|comments|docs|tests|config|build","severity":"critical|major|minor",
 "confidence":0-100,"file":"path","line":42,"lens":["bugs"],"title":"names the mechanism",
 "body":"trigger + consequence","fix":"concrete fix","class":"finding|open_question|pre_existing"}
  - class defaults to "finding"; source defaults to the filename suffix.
  - surface/severity/file/title are required. Everything else is optional.
  - "file" should be repo-relative ("scripts/facts.sh"). An absolute path still merges, but
    keeping one form across sources is what lets two reviewers' findings corroborate.

verdicts-<group>.jsonl — one per verifier:
{"id":"f03","verdict":"confirmed|refined|rejected|immaterial|pre_existing",
 "reason":"one clause","severity":"...","line":51}
  - on "refined", any field you include replaces the original; omitted fields stand.`;

const [, , cmd, ...rest] = process.argv;
const VALUE_FLAGS = ["sources-expected", "sim", "band", "window", "max-groups", "min-per-group"];
const flag = (name, dflt) => {
  const i = rest.indexOf(`--${name}`);
  return i === -1 ? dflt : rest[i + 1];
};
// Every tunable gates a rule, so a missing or non-numeric value must fail loudly: NaN makes
// every comparison false, silently disabling merging, LOW-SIM flagging and the drop rule.
const num = (name, dflt) => {
  // Absent is fine (use the default); present-with-no-value is not — `--sources-expected`
  // as the last word must not quietly become 3, which is a different run than the caller asked for.
  if (!rest.includes(`--${name}`)) return dflt;
  const raw = flag(name, undefined);
  const n = Number(raw);
  if (raw === undefined || raw === "" || raw.startsWith("--") || !Number.isFinite(n))
    die(`--${name} needs a numeric value (got ${raw === undefined ? "nothing" : `"${raw}"`})`);
  return n;
};
// First bare word that is not the value of a --flag.
let runDir;
for (let i = 0; i < rest.length; i++) {
  const a = rest[i];
  if (a.startsWith("--")) {
    if (VALUE_FLAGS.includes(a.slice(2))) i++;
    continue;
  }
  runDir = a;
  break;
}

switch (cmd) {
  case "schema":
    process.stdout.write(SCHEMA + "\n");
    break;
  case "validate": {
    if (!runDir) die("usage: findings.mjs validate <run-dir>");
    const files = runFiles(runDir, "findings-");
    if (!files.length) die(`no findings-*.jsonl in ${runDir}`, 1);
    let bad = 0;
    for (const f of files) {
      const { records, errors } = readJsonl(f);
      const probs = [...errors];
      records.forEach((r, i) => probs.push(...validate(r, `${basename(f)}#${i + 1}`)));
      bad += probs.length;
      process.stdout.write(`${basename(f)} n=${records.length} ${probs.length ? `errors=${probs.length}` : "ok"}\n`);
      probs.slice(0, 10).forEach((p) => process.stdout.write(`  ${p}\n`));
    }
    process.exit(bad ? 1 : 0);
  }
  case "reconcile":
    if (!runDir) die("usage: findings.mjs reconcile <run-dir> --sources-expected N");
    reconcile(runDir, {
      sourcesExpected: num("sources-expected", 3),
      sim: num("sim", 0.6),
      band: num("band", 0.3),
      window: num("window", 2),
    });
    break;
  case "group":
    if (!runDir) die("usage: findings.mjs group <run-dir>");
    group(runDir, { maxGroups: num("max-groups", 8), minPer: num("min-per-group", 3) });
    break;
  case "report":
    if (!runDir) die("usage: findings.mjs report <run-dir>");
    report(runDir);
    break;
  default:
    die("usage: findings.mjs schema|validate|reconcile|group|report <run-dir>");
}
