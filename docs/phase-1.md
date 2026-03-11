# ForgeZ — Phase 1

## Overview
Phase 1 delivers a personal CLI wrapper (`forge-zed`, aliased as `fz`) that orchestrates Worktrunk, GitHub CLI, and Claude Code into a single fast workflow helper.

## Command Specification

### Feature lifecycle

| Command | Description |
|---------|-------------|
| `fz start <branch-name> [--no-worktree]` | Create/switch feature branch, with or without worktree |
| `fz finish` | Local merge flow with clean-tree guardrail |
| `fz archive` | Remove feature branch/worktree that was already merged remotely |

### Daily workflow

| Command | Description |
|---------|-------------|
| `fz commit` | AI-assisted commit |
| `fz pr` | AI-assisted PR creation with clean-tree guardrail |

### Visibility

| Command | Description |
|---------|-------------|
| `fz list` | List active features/worktrees with branch and PR status |
| `fz log` | Show git log for current feature |
| `fz status` | Show PR status if a PR is attached to the current branch |

## Implementation

### Stack
**Bun + openTUI.** The `list`, `status`, and `log` commands require structured output formatting and `gh` JSON parsing that becomes awkward in bash. openTUI provides the rendering layer for these views and establishes the foundation for Phase 2's persistent multi-session workspace without requiring a rewrite.

### Tool integrations
- **Worktrunk** — branch/worktree flows
- **GitHub CLI** — remote operations (push/PR/status)
- **Claude Code** — agent-assisted commit and PR workflows
