use std::process::Command;

/// Run a git command and return trimmed stdout, or empty string on failure.
fn run(args: &[&str]) -> String {
    Command::new("git")
        .args(args)
        .output()
        .ok()
        .and_then(|o| String::from_utf8(o.stdout).ok())
        .map(|s| s.trim().to_string())
        .unwrap_or_default()
}

pub fn branch() -> String {
    let b = run(&["rev-parse", "--abbrev-ref", "HEAD"]);
    if b.is_empty() { "unknown".into() } else { b }
}

pub fn staged_count() -> usize {
    let s = run(&["diff", "--cached", "--name-only"]);
    s.lines().filter(|l| !l.is_empty()).count()
}

pub fn log_oneline() -> String {
    run(&["log", "-1", "--oneline"])
}

pub fn diff_stat() -> String {
    run(&["diff", "HEAD~1", "--stat", "--no-color"])
}

pub fn stage_all() {
    let _ = Command::new("git").args(["add", "-A"]).output();
}
