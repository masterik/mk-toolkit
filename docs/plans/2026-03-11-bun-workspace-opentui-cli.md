# Bun Workspace + OpenTUI CLI Package (fz)

## Context

The ForgeZ (`fz`) CLI is being migrated from a bash proof-of-concept (`fz-function.sh`) to a proper TypeScript/Bun application using `@opentui/react` as the rendering layer. Phase 1 establishes the Bun workspace structure and scaffolds the first package (`apps/cli`) using the `bun create tui --template react` template. The goal is a working `fz` binary (via `bun link`) that renders with openTUI React, replacing the existing shell function entirely.

## Decisions Made

- **Workspace path**: `apps/cli` (not `packages/cli` — apps/ reserved for runnables)
- **openTUI flavor**: `@opentui/react` (React reconciler)
- **Scaffolding**: `bun create tui --template react` (official template, minimal output)
- **Install method**: `bun link` for now (no `--compile` yet — native Zig FFI makes single-file executable complex)

## Final Structure

```
workflow_tool/
├── package.json           # root workspace config
├── tsconfig.json          # root base tsconfig (extended by app packages)
├── apps/
│   └── cli/               # scaffolded by bun create tui --template react
│       ├── package.json   # name: @fz/cli, bin: { fz: "src/index.tsx" }
│       ├── tsconfig.json  # from template (jsxImportSource: @opentui/react)
│       └── src/
│           ├── index.tsx  # entry: createCliRenderer + createRoot(<App />)
│           └── app.tsx    # <App /> — initial fz welcome placeholder
└── fz-function.sh         # REMOVE after bun link verified
```

## Steps

### 1. Create root `package.json`

Create `/Users/mk/Projects/workflow_tool/package.json`:

```json
{
  "name": "forge-zed",
  "private": true,
  "workspaces": ["apps/*"],
  "scripts": {
    "dev": "bun run apps/cli/src/index.tsx",
    "typecheck": "tsc --build"
  }
}
```

### 2. Create root `tsconfig.json`

Create `/Users/mk/Projects/workflow_tool/tsconfig.json`:

```json
{
  "compilerOptions": {
    "lib": ["ESNext"],
    "target": "ESNext",
    "module": "Preserve",
    "moduleDetection": "force",
    "moduleResolution": "bundler",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true
  }
}
```

### 3. Scaffold `apps/cli` using bun create tui

From `/Users/mk/Projects/workflow_tool/`:

```bash
bun create tui --template react --no-git --no-install apps/cli
```

This generates:
- `apps/cli/package.json` (name: `my-opentui-project`, deps: `@opentui/core`, `@opentui/react`, `react`)
- `apps/cli/tsconfig.json` (jsx: react-jsx, jsxImportSource: @opentui/react)
- `apps/cli/src/index.tsx` (minimal createCliRenderer + createRoot entry)
- `apps/cli/.gitignore`
- `apps/cli/README.md`

### 4. Update `apps/cli/package.json`

Modify the generated file to:

```json
{
  "name": "forge-zed-cli",
  "version": "0.1.0",
  "module": "src/index.tsx",
  "type": "module",
  "private": true,
  "bin": {
    "fx": "src/index.tsx",
    "forge-zed": "src/index.tsx"
  },
  "scripts": {
    "dev": "bun run --watch src/index.tsx",
    "typecheck": "tsc --noEmit"
  },
  "devDependencies": {
    "@types/bun": "latest"
  },
  "peerDependencies": {
    "typescript": "^5"
  },
  "dependencies": {
    "@opentui/core": "^0.1.87",
    "@opentui/react": "^0.1.87",
    "react": "^19.2.4"
  }
}
```

Key changes from template:
- `name`: `forge-zed-cli`
- `version`: `0.1.0`
- Added `bin.fx` and `bin.forge-zed` both pointing to `src/index.tsx`
- Removed the `"build": "bun build --compile..."` script (deferred)

### 5. Add `apps/cli/src/app.tsx`

Create the initial App component:

```tsx
import { TextAttributes } from "@opentui/core";

export function App() {
  return (
    <box alignItems="center" justifyContent="center" flexGrow={1}>
      <box justifyContent="center" alignItems="flex-end">
        <ascii-font font="tiny" text="fz" />
        <text attributes={TextAttributes.DIM}>ForgeZ — agentic workflow CLI</text>
      </box>
    </box>
  );
}
```

### 6. Update `apps/cli/src/index.tsx`

Replace the generated App inline definition with an import from `./app`:

```tsx
import { createCliRenderer } from "@opentui/core";
import { createRoot } from "@opentui/react";
import { App } from "./app";

const renderer = await createCliRenderer();
createRoot(renderer).render(<App />);
```

### 7. Install dependencies

From `/Users/mk/Projects/workflow_tool/`:

```bash
bun install
```

This installs all workspace deps including `apps/cli`'s openTUI packages.

### 8. Link the binary

From `/Users/mk/Projects/workflow_tool/apps/cli/`:

```bash
bun link
```

This registers `fz` globally. Bun will use the `bin.fz` field to execute `src/index.tsx`.

### 9. Remove fz-function.sh

Once `fz` binary is verified working, remove `fz-function.sh` from the shell config (`~/.zshrc` or wherever it's sourced) and delete the file.

## Critical Files

| File | Action |
|------|--------|
| `workflow_tool/package.json` | CREATE — root workspace config |
| `workflow_tool/tsconfig.json` | CREATE — base tsconfig |
| `workflow_tool/apps/cli/package.json` | SCAFFOLD then UPDATE |
| `workflow_tool/apps/cli/tsconfig.json` | SCAFFOLD (keep as-is) |
| `workflow_tool/apps/cli/src/index.tsx` | SCAFFOLD then UPDATE |
| `workflow_tool/apps/cli/src/app.tsx` | CREATE — initial fz App component |
| `workflow_tool/fz-function.sh` | DELETE after verification |

## Verification

1. After `bun install`, confirm no errors: `bun run typecheck`
2. Run directly: `bun run apps/cli/src/index.tsx` — should display openTUI React welcome screen
3. After `bun link`, run `fz` — should display same screen
4. Confirm `which fz` points to the linked binary
5. Confirm `fz` replaces `fz-function.sh` behavior (start, finish, commit, pr commands will be added in subsequent phases)

## Notes

- `bun build --compile` deferred: `@opentui/core` uses Zig FFI; native `.dylib` cannot be bundled into the single-file binary. Will revisit when openTUI's bundling story is clearer.
- `apps/cli/tsconfig.json` is kept separate from the root tsconfig (not extending it) since `jsxImportSource: @opentui/react` is app-specific and would pollute a base config.
- `bun.lock` generated by `bun create tui` in `apps/cli` will be replaced by the root lockfile after `bun install`.
