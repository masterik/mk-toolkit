# setup: migrating `.mkit/config.toml`

Written by the old `mkit init`. Read it as text — no tool needed — and carry over every value that differs
from its area's default, as that section's recommended answer:

| Old key | Goes to |
| --- | --- |
| `[commit] scopes`, `subject_max` | `commit.md` |
| `[review] mode` (not `full`) | `review.md` |
| `[review] reviewers` | `merge.md`, Reviewers |
| `[merge] style` (not `merge`) | `merge.md`, Merge style |
| `[remote] name` | `remote.md` |
| `[gate.commands]` (step → command) | `gate.md`, in the order `lint`, `typecheck`, `vet`, `test`, `build`, then the rest |
| `[cleanup] keep` | `cleanup.md`, Keep |
| `[spec] store`, `ref` | nothing — compare with `issue-tracker.md` and report a mismatch or absence |
| `version` | nothing |

Where a carried value equals what discovery finds today (`mkit init` copied discovered values in), say so
and recommend dropping it. A key not in this table: show it and ask. A file that won't read as TOML: show
the lines you could read, carry over nothing on a guess, and ask.

After the write, the report says `.mkit/config.toml` can go — `git rm .mkit/config.toml` (plain `rm` if it
was never committed); no skill reads it any more.
