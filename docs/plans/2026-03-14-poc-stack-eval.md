# ForgeZ POC — `fz commit` Tech Stack Evaluation

## Context

ForgeZ's current `fz commit` is a one-line shell delegation in `fz-function.sh:20`:
```bash
claude -p "/commit" --model="haiku" --dangerously-skip-permissions
```
The planned long-term stack is Bun + openTUI, but before committing we want to evaluate two compiled alternatives (Go + Charm, Rust + Ratatui). Each POC must exercise **rich styled output with colors/icons**, **interactive menu selection**, **subprocess streaming in a styled container**, and a **commit summary display**.

---

## Shared UX Flow (all three POCs)

```
┌──────────────────────────────────────┐
│  ⚡ ForgeZ — commit                   │
│  📍 main · 3 files staged            │
└──────────────────────────────────────┘

  How would you like to commit?

  ● 🚀 Commit all (stage everything + commit)
  ○ 📦 Commit staged only
  ○ 📝 Custom note (pass message to Claude)

  ⠋ Running claude commit...
  ┌─ claude output ─────────────────────┐
  │  Analyzing changes...               │
  │  Generated commit message:          │
  │  feat(cli): add commit POC          │
  │  ...                                │
  └─────────────────────────────────────┘

  ✅ Committed!
  ┌─ commit summary ───────────────────┐
  │  abc1234 feat(cli): add commit POC │
  │  3 files changed, +45 -12          │
  └────────────────────────────────────┘
```

### Step-by-step:

1. **Startup timer** — `[fz] ready in Xms` to stderr
2. **Styled header** — boxed banner with ⚡ icon, branch name (cyan), staged file count. Uses box-drawing characters, bold, and colors
3. **Interactive menu** — arrow-key navigable list with radio-button indicators (●/○) and icons:
   - 🚀 **Commit all** — runs `git add -A` before spawning Claude
   - 📦 **Commit staged only** — spawns Claude as-is (default, pre-selected)
   - 📝 **Custom note** — prompts for a message string, passes it to Claude via `-p "/commit <message>"`
4. **Spinner + boxed output** — while Claude runs, show a braille spinner. Capture Claude's stdout and render it inside a styled box with a dim border and title ("claude output"). Stream lines as they arrive
5. **Commit summary** — after Claude exits successfully, run `git log -1 --oneline` and `git diff HEAD~1 --stat --no-color`, display in a green-bordered summary box with ✅ icon. On failure, show ❌ with Claude's exit code
6. **Exit code** — propagate Claude's exit code

### What this exercises per stack:
- **Styled text:** colors, bold, dim, icons (emoji), box-drawing
- **Interactive input:** arrow-key menu navigation, enter to select, text input (custom note)
- **Async rendering:** spinner animation concurrent with subprocess output capture
- **Subprocess:** spawning, output streaming/capturing, exit code handling
- **Layout:** multiple styled boxes, sections with headers

---

## Implementation

### POC A: Bun (TypeScript)

**File:** `apps/cli/src/commit.tsx` (new, single file in existing workspace)

**Approach — openTUI React components + Bun APIs:**
- **Styled boxes:** `<box border borderStyle="rounded" borderColor="#00bcd4" title="...">` — declarative, no manual box-drawing
- **Menu:** `<select focused options={...} onSelect={handler}>` — built-in arrow-key nav, focus styling, descriptions
- **Custom note input:** `<input focused placeholder="..." onSubmit={handler}>` — built-in key handling, cursor, submit
- **Text styling:** `<b>`, `<span fg="..." attributes={TextAttributes.BOLD}>` + `TextAttributes.DIM` enum
- **Spinner:** React `useState` + `setInterval` cycling braille chars in a `<text>` node
- **Subprocess output capture:** `Bun.spawn()` with `stdout: "pipe"`, `ReadableStream` reader, `setState` to push lines into `<OutputBox>`
- **Git commands:** `Bun.spawn()` with `Response.text()` for capture
- **Startup:** `Bun.nanoseconds()`
- **Run:** `bun run apps/cli/src/commit.tsx`

### POC B: Go + Charm

**Directory:** `poc/go-charm/` (new)
**Files:** `go.mod`, `main.go`

**Approach — Bubble Tea + lipgloss + bubbles:**
- **Requires:** `brew install go` (not currently installed)
- **Styled boxes:** `lipgloss.NewStyle()` with `Border()`, `Foreground()`, `Bold()`. Lipgloss handles box rendering declaratively
- **Menu:** Bubble Tea model with `list` or manual key handling in `Update()`. Arrow keys move selection index, Enter confirms. View renders with ●/○ indicators via lipgloss styles
- **Custom note input:** `bubbles/textinput` component embedded in the Bubble Tea model
- **Spinner:** `bubbles/spinner` component — runs as a concurrent Bubble Tea command, auto-ticks
- **Subprocess + output capture:** `tea.Exec()` suspends Bubble Tea, hands terminal to Claude. Alternatively, capture output via pipe and render in the Bubble Tea view. `tea.Exec()` is cleaner but doesn't allow boxed output — so use pipe + model update approach
- **Subprocess output in box:** Run Claude with piped stdout, read lines in a goroutine, send them as `tea.Msg` to the model, render accumulated lines in a lipgloss-styled box
- **Git commands:** `exec.Command` with `.Output()` for capture
- **Startup:** `time.Now()` at top of `main()`
- **Run:** `cd poc/go-charm && go run .`
- **Build:** `go build -o fz-commit .`

### POC C: Rust + Ratatui

**Directory:** `poc/rust-ratatui/` (new)
**Files:** `Cargo.toml`, `src/main.rs`

**Approach — Ratatui immediate-mode TUI + crossterm backend:**
- **Requires:** Rust toolchain (`rustup`)
- **Render loop:** `Terminal::new(CrosstermBackend)` with `terminal.draw(|f| { ... })` callback, full-screen alternate screen
- **Layout:** `Layout::default().constraints([...]).split(f.area())` — responsive vertical splits for header/content/help
- **Styled boxes:** `Block::default().borders(Borders::ALL).border_type(BorderType::Rounded).border_style(...)` + `Paragraph::new(text).block(block)`
- **Menu:** `List::new(items)` rendered as `Widget` — manual `KeyCode` matching in event loop, re-rendered each frame
- **Custom note input:** Manual char-by-char in event loop with `f.set_cursor_position()` for blinking cursor
- **Spinner:** `AtomicBool` + frame counter ticking at 80ms in the main event loop (no separate thread needed — render loop handles it)
- **Subprocess output:** `Arc<Mutex<Vec<String>>>` shared between Claude thread and render loop, displayed via `Paragraph::new(lines).scroll((offset, 0))`
- **Help bar:** Custom `draw_help()` rendering styled `Span` pairs for keybindings
- **Git commands:** `Command::new("git").output()` for capture
- **Startup:** `Instant::now()`
- **Dependencies:** `crossterm = "0.28"`, `ratatui = "0.29"`
- **Run:** `cd poc/rust-ratatui && cargo run`
- **Build:** `cargo build --release`

### Housekeeping

- Add to `.gitignore`: `poc/go-charm/fz-commit`, `poc/rust-ratatui/target/`

---

## Execution Order

1. **POC A** — create `apps/cli/src/commit.ts` (zero setup, workspace exists)
2. **Install Go** — `brew install go`
3. **POC B** — `mkdir -p poc/go-charm`, init module, write `main.go`, `go mod tidy`
4. **Install Rust** — `rustup`
5. **POC C** — `mkdir -p poc/rust-ratatui`, `cargo init`, add crossterm, write `src/main.rs`
6. **Test all three** — stage changes, run each POC, record metrics
7. **Write comparison table** to stdout

---

## Comparison Table (measured 2026-03-14)

| Metric                   | Bun (TS)                     | Go + Charm                 | Rust + Ratatui                  |
| ------------------------ | ---------------------------- | -------------------------- | ------------------------------- |
| Lines of code            | 204 (app) + 80 (bootstrap)   | 620                        | 564                             |
| Startup time (measured)  | 2 ms                         | TBD (interactive)          | TBD (interactive)               |
| Cold build time          | 0 (interpreted)              | 0.5 s                      | 8.6 s                           |
| Binary size              | N/A (needs bun)              | 4.4 MB                     | 0.9 MB                          |
| Styled box rendering     | `<box border>` (declarative) | lipgloss (declarative)     | Block + Paragraph (declarative) |
| Menu interaction         | `<select>` component         | key.Binding + Bubble Tea   | List widget + KeyCode match     |
| Spinner / async UI       | React useState + setInterval | bubbles/spinner + tea.Cmd  | Thread + AtomicBool + tick      |
| Output capture + display | ReadableStream + setState    | viewport + tea.Msg stream  | Paragraph::scroll + Mutex       |
| Text input               | `<input>` component          | bubbles/textinput          | Manual char-by-char + cursor    |
| Responsive layout        | openTUI flexbox              | WindowSizeMsg + adaptive   | Layout::split(f.area())         |
| Help display             | N/A                          | bubbles/help (auto-gen)    | Custom draw_help()              |
| Theme awareness          | Hex colors                   | AdaptiveColor (light/dark) | Hardcoded ANSI-16               |
| TUI library maturity     | openTUI (early, 0.1.x)       | Bubble Tea (mature, v1)    | Ratatui (mature, v0.29)         |
| Distribution             | Needs bun runtime            | Single static binary       | Single static binary            |

---

## Verification

1. Stage changes in this repo (or a test repo)
2. Run each POC and verify:
   - Styled header with colors and icons renders correctly
   - Arrow keys navigate the menu, Enter selects
   - "Commit all" stages everything before spawning Claude
   - "Custom note" prompts for text input
   - Spinner animates while Claude runs
   - Claude output appears inside a styled box
   - Commit summary shows hash, message, and diffstat in a green box on success
   - ❌ error display on failure
3. Exit code: `<poc-command>; echo $?` matches Claude's exit code
4. Compare `[fz] ready in Xms` across all three
5. Subjective: code readability, framework friction, "feels like home" factor
