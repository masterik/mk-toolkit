use std::io::BufRead;
use std::process::{Command, Stdio};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};

/// Runs the Claude CLI subprocess, streaming stdout line-by-line into shared state.
/// Intended to be called from a background thread.
pub fn run_subprocess(
    prompt: &str,
    lines: Arc<Mutex<Vec<String>>>,
    done: Arc<AtomicBool>,
    exit_code: Arc<Mutex<i32>>,
) {
    let result = Command::new("claude")
        .args(["-p", prompt, "--model=haiku", "--dangerously-skip-permissions"])
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn();

    let mut child = match result {
        Ok(c) => c,
        Err(_) => {
            if let Ok(mut ec) = exit_code.lock() {
                *ec = 1;
            }
            done.store(true, Ordering::Relaxed);
            return;
        }
    };

    if let Some(stdout) = child.stdout.take() {
        let reader = std::io::BufReader::new(stdout);
        for line in reader.lines().map_while(Result::ok) {
            if let Ok(mut buf) = lines.lock() {
                buf.push(line);
            }
        }
    }

    let code = child.wait().ok().and_then(|s| s.code()).unwrap_or(1);
    if let Ok(mut ec) = exit_code.lock() {
        *ec = code;
    }
    done.store(true, Ordering::Relaxed);
}
