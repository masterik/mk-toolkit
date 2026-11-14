# Quality gate

mkit's `pr` and `finish` run these commands before opening a pull request or merging: in this order, from
the repo root, stopping at the first failure. This list replaces mkit's detection (a task-runner `check` or
`ci` target, else each ecosystem's lint, test and build).

1. `<command>`
2. `<command>`

Delete this file to go back to detection.
