# ForgeZ — Concept

## Summary
`forge-zed` (ForgeZ) starts as a personal workflow helper around tools that already work well: Worktrunk, GitHub CLI, and Claude Code (with other agents/tools evaluated over time).
The goal is faster iteration with fewer manual steps, then gradual evolution into a local-first app.

## Phase 1: Personal Wrapper on Existing Tools
Phase 1 is intentionally narrow: this is a wrapper/helper for my own workflow, not a general platform.

### Positioning
- Thin orchestration layer on top of existing CLIs.
- No reinvention of Git hosting, branching, or agent execution.
- Practical defaults tuned for your habits.

### Tooling baseline
- Worktrunk for branch/worktree flows.
- GitHub CLI for remote operations (push/PR/status).
- Claude Code for agent-assisted actions (commit/PR workflows).
- Other agents can be evaluated and plugged in later.

See [prd-phase-1.md](./prd-phase-1.md) for command specification and implementation details.

## Phase 2: Local App (Desktop or TUI)
Phase 2 expands from helper to workspace control plane, similar in spirit to the Codex macOS app but centered on Claude Code.

### Core capabilities
- Multiple projects.
- Multiple sessions per project.
- Local execution and parallel runs.
- Worktree-aware session management.
- Quick open in editor.
- One-click commit, push, and create PR.
- Diff/changes view.
- Built-in terminal.

### Key product decision to evaluate
- Main agent panel architecture:
  - Terminal-driven UX.
  - API/SDK-driven UX (Codex-style orchestration).

### Fast run/play loops
- Quick run/play actions in browser and/or terminal.

## Phase 3: Integrated Runtime Surface
Phase 3 adds deeper built-in tooling for development feedback loops.

- Integrated browser (similar to cmux-style embedded browser workflows).
- Integrated mobile preview/simulator.
- Additional integrated surfaces as needed (based on daily workflow bottlenecks).

## Design Principles
- **Personal-first utility:** optimize for your workflow before generalization.
- **Composition over replacement:** wrap strong existing tools instead of rebuilding them.
- **Local and parallel by default:** multiple active sessions/worktrees should be normal.
- **Progressive enhancement:** script wrapper first, then richer UI/runtime layers.
- **Fast execution loops:** minimize friction from idea to branch to PR.
