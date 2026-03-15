# POC Tech Stack Evaluation — ForgeZ `fz commit`

> **Date:** 2026-03-15
> **Methodology:** Each POC was refactored to follow its ecosystem's best practices, then independently reviewed by a specialist. Metrics were measured post-refactor.

---

## 1. Comparison Table

| Metric                  | Bun + openTUI/React (A) | Go + Charm (B)                   | Rust + Ratatui (C)     | Bun + openTUI/core (D) | Bun + Rezi (E)                    |
| ----------------------- | ----------------------- | -------------------------------- | ---------------------- | ---------------------- | --------------------------------- |
| **Lines of code**       | 375                     | 719                              | 704                    | 552                    | 427                               |
| **Source files**        | 11                      | 7                                | 6                      | 4                      | 5                                 |
| **Startup time**        | ~50 ms                  | 44 ms                            | <1 ms                  | ~50 ms                 | ~33 ms                            |
| **Cold build time**     | N/A (interpreted)       | 0.38 s                           | 7.4 s                  | N/A (interpreted)      | N/A (interpreted)                 |
| **Binary size**         | N/A (requires Bun)      | 4,531 KB                         | 917 KB                 | N/A (requires Bun)     | N/A (requires Bun + native addon) |
| **Dependencies**        | 3 (core, react, react)  | 3 (bubbletea, bubbles, lipgloss) | 2 (ratatui, crossterm) | 1 (@opentui/core)      | 2 (@rezi-ui/core, @rezi-ui/node)  |
| **Runtime requirement** | Bun ≥ 1.x               | None (static binary)             | None (static binary)   | Bun ≥ 1.x              | Bun ≥ 1.x                         |
| **Review score**        | 3 / 5                   | 4 / 5                            | 3 / 5                  | 3.5 / 5                | 4 / 5                             |

---

## 2. Refactor Summary

### Bun + openTUI (TypeScript/React)
**Structure:** Monolithic `commit.tsx` (285 lines) → 11 files across `components/`, `hooks/`, `utils/`, `types.ts`.

**Improvements made:**
- Discriminated union for phase state (MenuPhase | NoteInputPhase | RunningPhase | SummaryPhase)
- Extracted reusable components: Header, OutputBox, Spinner, Summary
- Custom `useSubprocess` hook encapsulating Claude spawn and stream reading
- Git helpers extracted to `utils/git.ts`
- Added `@types/react` for proper type inference

**Reviewer assessment (3/5):**
The file decomposition follows standard React conventions, but the refactor introduced a dual-phase-tracking anti-pattern where `CommitApp` maintains both a `localPhase` and the subprocess phase, defeating the discriminated union's single-source-of-truth intent. The `as never` type cast on `onSubmit` is a type-safety regression caused by an upstream openTUI/React typing conflict. The `useSubprocess` hook conflates subprocess lifecycle with UI navigation — it knows about "menu" and "note-input" phases that are not subprocess concerns.

**Remaining issues:** subprocess not cleaned up on unmount, git errors silently swallowed, array index used as React key in OutputBox.

### Go + Charm (Bubble Tea + lipgloss)
**Structure:** Monolithic `main.go` (621 lines) → 7 files across `internal/tui/`, `internal/git/`, `internal/claude/`.

**Improvements made:**
- Standard `internal/` package layout with clean separation of concerns
- Godoc comments on all exported types and functions
- Named constants replacing magic numbers (defaultWidth, maxBoxWidth, textInputCharLimit)
- Proper error wrapping with `fmt.Errorf` and `%w`
- Bubble Tea Model/Update/View contract preserved across file boundaries

**Reviewer assessment (4/5):**
Best-structured of the three refactors. The `internal/` layout is idiomatic Go, and the Bubble Tea architecture maps naturally to a multi-file split. The phase-specific `updateMenu`/`viewMenu` pattern keeps each file focused. Main weakness is silent error handling — git helpers return fallback values on error, and `claude.Run` surfaces subprocess failures as opaque exit codes with no diagnostic. The double-pointer pattern for sharing `tea.Program` is unusual but functional. Missing subprocess cancellation via `context.Context`.

**Remaining issues:** no `context.Context` for subprocess cancellation, `scanner.Err()` unchecked, mutable package-level style vars, emoji as Unicode escapes.

### Rust + Ratatui (crossterm)
**Structure:** Monolithic `main.rs` (565 lines) → 6 files: `app.rs`, `ui.rs`, `git.rs`, `claude.rs`, `input.rs`.

**Improvements made:**
- `App` struct with proper encapsulation and `impl` block
- `ClaudeState` abstraction wrapping `Arc<Mutex<>>` / `Arc<AtomicBool>`
- `InputResult` enum eliminating boolean/break control flow
- Named constants: `BRAILLE_FRAMES`, `POLL_INTERVAL_MS`, layout heights
- `.unwrap()` on mutex locks replaced with `if let Ok()` guards
- Passes `cargo clippy -- -D warnings` cleanly

**Reviewer assessment (3/5):**
Good module boundaries and enum-based state machine, but several correctness gaps. Terminal raw mode can leak if `main()` errors after `enable_raw_mode()` — no RAII cleanup guard. `std::process::exit()` skips destructors. The spawned Claude thread's `JoinHandle` is discarded, so the subprocess is orphaned on quit. Lock handling is inconsistent between modules. `note.len()` assumes ASCII for cursor positioning. Output-box rendering logic is duplicated between `draw_output_box` and `draw_summary_phase`.

**Remaining issues:** no terminal cleanup on error, orphaned subprocess, Ctrl+C not handled in NoteInput phase, `Mutex<i32>` should be `AtomicI32`, no `Default` derive on `ClaudeState`.

### Bun + openTUI/core (imperative, no React)
**Structure:** 4 files: `main.ts`, `ui.ts` (427 lines), `git.ts`, `claude.ts`. Total 552 LOC.

**Architecture:**
Imperative TUI rendering using `@opentui/core` directly — no JSX, no React, no `@opentui/react`. All renderables (`BoxRenderable`, `TextRenderable`, `SelectRenderable`, `InputRenderable`) are plain classes instantiated via `new XRenderable(ctx, options)`. Phase transitions call `clearRoot()` then build a new tree of renderables. Key events handled through `renderer.keyInput.on("keypress", handler)` and renderable-level event emitters (`SelectRenderableEvents.ITEM_SELECTED`, `InputRenderableEvents.ENTER`).

**Key findings:**
- `@opentui/core` fully supports imperative usage — React is genuinely optional
- Eliminates the `as never` type cast and dual-phase-tracking anti-pattern from POC A
- Single dependency (`@opentui/core`) vs three in POC A
- No discriminated union needed — simple string phase + mutable `AppState` object
- `clearRoot()` + rebuild pattern is simple but causes full re-renders (acceptable for this complexity)

**Reviewer assessment (3.5/5):**
Cleaner than the React version. The imperative API maps well to a phase-based state machine — each phase function builds its own widget tree, no component lifecycle complexity. The `clearRoot()` approach trades render efficiency for code simplicity, which is the right trade-off for a POC. Main weakness: `process.exit()` in key handlers skips any cleanup. The 552 LOC is higher than POC A's 375 because imperative node creation is more verbose than JSX, but there's no hidden complexity in hooks or lifecycle.

**Remaining issues:** `process.exit()` in summary handler bypasses cleanup, no subprocess cancellation on quit, spinner timer not cleared if user quits during running phase.

### Bun + Rezi (deterministic rendering engine)
**Structure:** 5 files: `main.ts` (246 lines), `types.ts`, `git.ts`, `claude.ts`. Total 427 LOC.

**Architecture:**
Rezi uses a state-driven `app.view(state => ...)` pattern with a native C rendering engine (Zireael). The architecture is Elm-like: define initial state, a view function that returns a widget tree from state, and key handlers that call `app.update()` to modify state and trigger re-renders. The `ui.*` API (`ui.page`, `ui.column`, `ui.box`, `ui.text`, `ui.input`, `ui.header`) is declarative and composable. Phase rendering uses a `switch` statement dispatching to per-phase view functions.

**Key findings:**
- Lowest LOC of all five POCs (427) — the `ui.*` API is remarkably concise
- Elm-like state management is clean: `app.update(prev => newState)` with immutable state
- `app.keys()` provides centralized keybinding with access to `{ state, update }` context
- Built-in theming with 6 themes (dark, light, nord, dracula, etc.) — no manual color constants needed
- Native C rendering engine gives ~33ms startup — faster than Bun+openTUI despite also requiring Bun
- `app.run()` (not `app.start()`) is the correct entry point — minor API discovery needed
- No `getState()` method — state only accessible inside `view()` and `keys()` callbacks, which forces clean patterns but makes subprocess integration slightly awkward (needs `setTimeout` to defer spawn)

**Reviewer assessment (4/5):**
The best developer experience of the TypeScript options. The Elm-like architecture naturally prevents the dual-state-tracking problem that plagued POC A. The `ui.*` API is the most readable of all five POCs — `ui.page({ header, body })` is self-documenting. The `app.keys()` centralization avoids scattered event listeners. The `setTimeout(() => spawnClaude(...), 0)` workaround for deferring subprocess spawn after state commit is slightly inelegant but functional. The native rendering engine is a genuine differentiator for perceived responsiveness.

**Remaining issues:** subprocess not cancelled on quit, `setTimeout` hack for spawn deferral, `app.update()` inside subprocess callbacks could race with key handlers (no built-in concurrency guard), requires Bun + native addon (Zireael C engine).

---

## 3. Trade-off Analysis

### Startup Speed
Rust dominates at <1 ms. Go is fast at 44 ms. Rezi is surprisingly fast at ~33 ms despite running on Bun, thanks to its native C rendering engine. openTUI variants are ~50 ms. All are acceptable for interactive use.

**Winner: Rust** — sub-millisecond startup is effectively instant. **Runner-up: Rezi** — fastest of the TS options.

### Binary Portability
Go and Rust produce self-contained binaries requiring no runtime. Rust's binary is 5× smaller (917 KB vs 4.5 MB). All three TS options require Bun; Rezi additionally bundles a native C addon (Zireael).

**Winner: Rust** — smallest binary, no runtime dependency, compiles for any target triple.

### Developer Experience (DX)
Rezi has the lowest LOC (427) with the most readable API (`ui.page({ header, body })`). openTUI/React is next at 375 LOC but with hidden complexity in hooks. openTUI/core is 552 LOC — imperative node creation is more verbose than JSX. Go and Rust hover around 700 LOC. Hot reload via `bun run --watch` gives all TS options the fastest feedback loop. Go's compile time (0.38 s) is nearly instant. Rust's 7.4 s cold build is the slowest.

**Winner: Rezi** — lowest LOC, most readable API, fastest iteration. Go is a close second for compiled languages.

### Ecosystem Maturity (TUI)
- **Bubble Tea** (Go): Most mature TUI framework. Rich component library (bubbles), consistent API, large community. The Elm architecture is well-understood.
- **Ratatui** (Rust): Active and growing. Immediate-mode rendering is powerful but requires more manual state management.
- **openTUI** (TS): Young but functional. React layer has typing issues; core imperative API is cleaner but less documented.
- **Rezi** (TS): Newest of all. 56 built-in widgets, native rendering engine, built-in theming. Alpha-stage (`0.1.0-alpha.60`) — impressive feature set but least battle-tested.

**Winner: Go (Charm)** — most mature, best component library. **Watch: Rezi** — if it stabilizes, its widget count and native engine are compelling.

### Code Quality Ceiling
Go (4/5) and Rezi (4/5) tied for highest. Both use Elm-like state machines that naturally prevent state management mistakes. openTUI/core (3.5/5) improved over the React version by eliminating the dual-phase anti-pattern. Rust (3/5) and openTUI/React (3/5) had the most correctness gaps.

**Winner: Go and Rezi (tied)** — Elm-like architectures provide guardrails that prevent common state management mistakes.

---

## 4. Conclusion

### Recommendation: **Go + Charm**

For ForgeZ's needs — a personal CLI tool that must start fast, distribute as a single binary, and be pleasant to develop — **Go + Charm** offers the best overall balance across all five POCs:

| Factor                | Weight | TS/React (A) | Go (B) | Rust (C) | TS/core (D) | Rezi (E) |
| --------------------- | ------ | ------------ | ------ | -------- | ----------- | -------- |
| Startup speed         | High   | ⬤⬤           | ⬤⬤⬤    | ⬤⬤⬤⬤⬤    | ⬤⬤          | ⬤⬤⬤      |
| Binary portability    | High   | ⬤            | ⬤⬤⬤    | ⬤⬤⬤⬤⬤    | ⬤           | ⬤        |
| DX / iteration speed  | High   | ⬤⬤⬤⬤         | ⬤⬤⬤⬤   | ⬤⬤       | ⬤⬤⬤         | ⬤⬤⬤⬤⬤    |
| Ecosystem maturity    | Medium | ⬤⬤           | ⬤⬤⬤⬤⬤  | ⬤⬤⬤      | ⬤⬤          | ⬤        |
| Code quality ceiling  | Medium | ⬤⬤⬤          | ⬤⬤⬤⬤   | ⬤⬤⬤      | ⬤⬤⬤         | ⬤⬤⬤⬤     |
| LOC / maintainability | Low    | ⬤⬤⬤⬤⬤        | ⬤⬤⬤    | ⬤⬤⬤      | ⬤⬤⬤         | ⬤⬤⬤⬤⬤    |

### What the new POCs revealed

**POC D (openTUI/core):** Confirmed that React is genuinely optional — `@opentui/core`'s imperative API is clean and complete. Eliminating React removed the typing conflicts and dual-phase anti-pattern. However, imperative node creation is verbose (552 LOC vs 375 with JSX), and the `clearRoot()` + rebuild pattern, while simple, doesn't scale well. The single-dependency story is good but the DX improvement over React is marginal.

**POC E (Rezi):** The standout surprise. Lowest LOC (427), most readable code (`ui.page({ header, body })`), fastest TS startup (~33ms), and an Elm-like state model that naturally prevented the state management issues seen in POCs A and D. The native C rendering engine (Zireael) is a genuine differentiator. The 56 built-in widgets and 6 themes mean less custom code. **However**, Rezi is alpha-stage software (`0.1.0-alpha.60`). For a personal tool this is acceptable risk; for production use it would not be.

### Why Go still wins

**Why not Rezi (despite best DX)?** Rezi has the best developer experience of all five options, but three factors keep it from the top: (1) alpha-stage maturity — API could change, bugs likely in edge cases; (2) requires Bun + native C addon — heavier runtime dependency than Go's single binary; (3) no `getState()` outside callbacks forces workarounds for async patterns. If Rezi reaches 1.0, this recommendation should be revisited.

**Why not Rust?** Rust wins on raw performance metrics (startup, binary size), but the 7.4 s cold build time significantly slows iteration. The correctness bar for terminal lifecycle management is higher (RAII cleanup, thread joining, subprocess killing). For a personal tool where development velocity matters, the trade-off favors Go.

**Why not openTUI (either variant)?** React version has typing conflicts and state complexity. Core version eliminates those but is verbose and less well-documented. Neither offers advantages over Go for a CLI binary.

**Why Go?** Go hits the sweet spot: near-instant compilation (0.38 s), self-contained binaries, the most mature TUI framework (Charm), and an Elm-like architecture that naturally produces clean code. The 44 ms startup is fast enough. The higher LOC reflects Go's explicit style — a feature for long-term maintenance.

### Next Steps
1. Adopt Go + Charm as the implementation stack for ForgeZ Phase 1
2. Address reviewer findings: add `context.Context` for subprocess cancellation, surface errors properly, replace double-pointer `tea.Program` pattern
3. Expand the Go POC into the full `fz` command structure per `docs/phase-1.md`
4. **Monitor Rezi** — if it reaches stable release, reconsider for future TypeScript-based tooling where binary distribution is not required
