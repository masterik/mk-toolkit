# ── Feature workflow ────────────────────────────────────────────────────────
# Usage: feat start <name> | feat finish | feat commit | feat pr
feat() {
  case "$1" in
    start)
      if [[ -z "$2" ]]; then
        echo "Usage: feat start <branch-name>"
        return 1
      fi
      wt switch --create "$2" --base @
      ;;
    finish)
      if [[ -n "$(git status --porcelain)" ]]; then
        echo "Uncommitted changes detected. Please commit first (feat commit)"
        return 1
      fi
      wt merge --no-squash
      ;;
    commit)
      claude -p "/commit" --model="haiku" --dangerously-skip-permissions
      ;;
    pr)
      if [[ -n "$(git status --porcelain)" ]]; then
        echo "Uncommitted changes detected. Please commit first (feat commit)"
        return 1
      fi
      claude -p "/core:create-pr" --model="haiku" --dangerously-skip-permissions
      ;;
    *)
      echo "Usage: feat <start <name>|finish|commit|pr>"
      return 1
      ;;
  esac
}
