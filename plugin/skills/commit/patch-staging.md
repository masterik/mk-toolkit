# commit: patch staging, non-interactively

For a file whose changes belong in more than one commit. Per mixed file, in `<run-dir>`:

```bash
# 1. cut the file's unstaged diff into a fresh <patch> = <run-dir>/patch-<n>.diff (<n> = 1, 2, …; never
#    reused, never named after the source file). git writes it: a shell `>` breaks it under PowerShell
"<git>" -C "<toplevel>" diff --output="<patch>" -- "<path>"

# 2. edit <patch>: delete whole `@@ … @@` blocks that belong in a later commit; keep the header lines;
#    never renumber or hand-edit a `@@` line

# 3. apply what is left to the index only
"<git>" -C "<toplevel>" apply --cached -- "<patch>"

# 4. verify, always
"<git>" -C "<toplevel>" diff --cached --stat -- "<path>"
```

- `--cached` touches only the index; the working tree keeps every hunk. Step 3 fails → nothing staged:
  re-cut from step 1.
- **Hunks that share lines are not split.** A split needing a cut inside a hunk → commit the file whole and
  say so in the report.
- Undo: `"<git>" -C "<toplevel>" apply --cached -R -- "<patch>"`; a whole file: `"<git>" -C "<toplevel>" restore --staged --
  "<path>"`. Delete the patch once step 4 verified.
