# PowerShell (Windows)

Read when your shell is PowerShell: Codex and Copilot on Windows, and Claude Code without Git Bash. Under
Git Bash (Claude Code's Bash tool on Windows) none of this applies: run `bash "<script>"` with the path
quoted and in forward slashes.

## Running the bundled scripts

`facts.sh` and `branch-status.sh` need Git Bash. A bare `bash` from PowerShell is often missing (Git for
Windows puts only `Git\cmd` on `PATH`) or WSL's, which runs the script against Linux `git` and `/mnt/c`
paths — wrong output that reads right. So never a bare `bash`; resolve Git's own once per session:

1. `git --exec-path` prints `<git-root>/<arch>/libexec/git-core` (e.g. `C:/Program Files/Git/mingw64/libexec/git-core`).
   `<bash>` is `<git-root>/bin/bash.exe`: drop the last three parts, add `bin/bash.exe`.
2. Run every script as `& "<bash>" "<script>" <args>` — the `&` and both quotes are required.
3. **`<bash>` does not exist** (MinGit, a `git` not from Git for Windows) → stop, and tell the user: the
   skill needs Git Bash; install Git for Windows (`winget install Git.Git`), then run it again. Never derive
   the facts with other calls, never use WSL's `bash`.

The scripts print Windows paths (`C:/…`); use them as given.

## Commands

- **A quoted path that starts a command needs `&`**: `& "<git>" -C "<toplevel>" status --porcelain`.
  `<git>` is the `Source` of `Get-Command git` (there is no `command -v`).
- **Single-quote an argument holding `@` `{` `}` `$` or `` ` ``**: `'@{upstream}'` — unquoted, `@{…}` is a
  hashtable and the command does not parse.
- **Never redirect a command's output into a file the next command reads** (`>`): Windows PowerShell 5.1
  writes UTF-16, which `git apply` rejects. Let the tool write it (`git diff --output="<file>"`), or write it
  with your file tool.
- `$TMPDIR` is `$env:TEMP`; prefer `<run-dir>`.
