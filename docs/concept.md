# Feature Workflow Tool Concept

## Summary
`feat` starts as a personal workflow helper around tools that already work well: Worktrunk, GitHub CLI, and Claude Code (with other agents/tools evaluated over time).
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

### Command model

**Feature lifecycle:**
- `feat start <branch-name> [--no-worktree]`: create/switch feature branch, with or without worktree.
- `feat finish`: local merge flow with clean-tree guardrail.
- `feat archive`: remove feature branch/worktree that was already merged remotely.

**Daily workflow:**
- `feat commit`: AI-assisted commit.
- `feat pr`: AI-assisted PR creation with clean-tree guardrail.

**Visibility:**
- `feat list`: list active features/worktrees with branch and PR status.
- `feat log`: show git log for current feature.
- `feat status`: show PR status if a PR is attached to the current branch.

### Tooling decision
Phase 1 uses **Bun + openTUI**. The `list`, `status`, and `log` commands require structured output formatting and `gh` JSON parsing that becomes awkward in bash. openTUI provides the rendering layer for these views and establishes the foundation for Phase 2's persistent multi-session workspace without requiring a rewrite.

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
