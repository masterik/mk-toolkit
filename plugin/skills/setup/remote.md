# Remote

**Remote: `<name>`.** mkit's `pr` and `finish` push branches to this remote and open and merge pull requests
in its GitHub repository; `cleanup` fetches from it and looks up pull requests there.

A clone without a remote of that name falls back to `origin`, then to its only remote, and says so. Delete
this file to go back to that order, with a question when several remotes remain.
