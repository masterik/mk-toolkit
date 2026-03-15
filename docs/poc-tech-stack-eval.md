# POC Tech Stack Evaluation — ForgeZ `fz commit`

> **Date:** 2026-03-15
> **Methodology:** Each POC was refactored to follow its ecosystem's best practices, then independently reviewed by a specialist. Metrics were measured post-refactor.

---

## 1. Comparison Table

| Metric                  | Bun + openTUI (TS)                     | Go + Charm                       | Rust + Ratatui         |
| ----------------------- | -------------------------------------- | -------------------------------- | ---------------------- |
| **Lines of code**       | 375                                    | 719                              | 704                    |
| **Source files**        | 11                                     | 7                                | 6                      |
| **Startup time**        | ~50 ms                                 | 44 ms                            | <1 ms                  |
| **Cold build time**     | N/A (interpreted)                      | 0.38 s                           | 7.4 s                  |
| **Binary size**         | N/A (requires Bun runtime)             | 4,531 KB                         | 917 KB                 |
| **Dependencies**        | 3 (openTUI core, openTUI react, react) | 3 (bubbletea, bubbles, lipgloss) | 2 (ratatui, crossterm) |
| **Runtime requirement** | Bun ≥ 1.x                              | None (static binary)             | None (static binary)   |
| **Review score**        | 3 / 5                                  | 4 / 5                            | 3 / 5                  |

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

---

## 3. Trade-off Analysis

### Startup Speed
Rust dominates at <1 ms. Go is fast at 44 ms. Bun is acceptable at ~50 ms but requires the Bun runtime to be installed — cold-start of the runtime itself adds latency on first invocation.

**Winner: Rust** — sub-millisecond startup is effectively instant and matters for a CLI tool invoked frequently.

### Binary Portability
Go and Rust produce self-contained binaries requiring no runtime. Rust's binary is 5× smaller (917 KB vs 4.5 MB). Bun requires the user to have the Bun runtime installed.

**Winner: Rust** — smallest binary, no runtime dependency, compiles for any target triple.

### Developer Experience (DX)
TypeScript/React has the lowest LOC (375 vs ~700) and the most familiar paradigm for web developers. Hot reload via `bun run --watch` provides the fastest feedback loop. Go's compile time (0.38 s) is nearly instant. Rust's 7.4 s cold build is the slowest, though incremental builds are fast.

**Winner: TypeScript** — lowest LOC, fastest iteration, most transferable skills. Go is a close second.

### Ecosystem Maturity (TUI)
- **Bubble Tea** (Go): Most mature TUI framework. Rich component library (bubbles), consistent API, large community. The Elm architecture is well-understood and maps cleanly to multi-file projects.
- **Ratatui** (Rust): Active and growing. Immediate-mode rendering is powerful but requires more manual state management. Smaller component ecosystem than Charm.
- **openTUI** (TS): Newest of the three. React model is powerful but the library has upstream typing issues (openTUI + @types/react conflicts). Smaller community, less battle-tested.

**Winner: Go (Charm)** — most mature, best component library, cleanest architecture pattern.

### Code Quality Ceiling
The Go refactor scored highest (4/5), suggesting the Bubble Tea architecture naturally guides developers toward clean code. The TS and Rust versions both scored 3/5 — TS due to state management complexity, Rust due to correctness gaps in terminal/subprocess lifecycle.

**Winner: Go** — the Elm architecture provides guardrails that prevent common state management mistakes.

---

## 4. Conclusion

### Recommendation: **Go + Charm**

For ForgeZ's needs — a personal CLI tool that must start fast, distribute as a single binary, and be pleasant to develop — **Go + Charm** offers the best overall balance:

| Factor                | Weight | TS    | Go    | Rust  |
| --------------------- | ------ | ----- | ----- | ----- |
| Startup speed         | High   | ⬤⬤    | ⬤⬤⬤   | ⬤⬤⬤⬤⬤ |
| Binary portability    | High   | ⬤     | ⬤⬤⬤   | ⬤⬤⬤⬤⬤ |
| DX / iteration speed  | High   | ⬤⬤⬤⬤⬤ | ⬤⬤⬤⬤  | ⬤⬤    |
| Ecosystem maturity    | Medium | ⬤⬤    | ⬤⬤⬤⬤⬤ | ⬤⬤⬤   |
| Code quality ceiling  | Medium | ⬤⬤⬤   | ⬤⬤⬤⬤  | ⬤⬤⬤   |
| LOC / maintainability | Low    | ⬤⬤⬤⬤⬤ | ⬤⬤⬤   | ⬤⬤⬤   |

**Why not Rust?** Rust wins on raw performance metrics (startup, binary size), but the 7.4 s cold build time significantly slows iteration during active development. The TUI ecosystem is less mature, and the correctness bar for terminal lifecycle management is higher (RAII cleanup, thread joining, subprocess killing). For a personal tool where development velocity matters more than shaving 40 ms off startup, the trade-off favors Go.

**Why not TypeScript?** TypeScript has the best DX for prototyping, but requiring the Bun runtime is a distribution friction point. The openTUI library has upstream typing issues, and the React paradigm — while powerful — introduced the most complex state management problems in this evaluation. If ForgeZ were a web tool, TS would win; for a CLI binary, it's the weakest choice.

**Why Go?** Go hits the sweet spot: near-instant compilation (0.38 s), self-contained binaries, the most mature TUI framework (Charm), and an architecture (Bubble Tea's Elm model) that naturally produces clean, maintainable code. The 44 ms startup is fast enough for interactive use. The slightly higher LOC compared to TS reflects Go's explicit style, which is a feature for long-term maintenance.

### Next Steps
1. Adopt Go + Charm as the implementation stack for ForgeZ Phase 1
2. Address reviewer findings: add `context.Context` for subprocess cancellation, surface errors properly, replace double-pointer `tea.Program` pattern
3. Expand the Go POC into the full `fz` command structure per `docs/phase-1.md`
