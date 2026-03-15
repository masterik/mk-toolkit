use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};

use crate::claude;
use crate::git;

// ── Constants ────────────────────────────────────────────────────────────────

pub const BRAILLE_FRAMES: &[char] = &['\u{280B}', '\u{2819}', '\u{2839}', '\u{2838}', '\u{283C}', '\u{2834}', '\u{2826}', '\u{2827}', '\u{2807}', '\u{280F}'];

pub const POLL_INTERVAL_MS: u64 = 80;

// ── Phase & Choice enums ─────────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Phase {
    Menu,
    NoteInput,
    Running,
    Summary,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Choice {
    CommitAll,
    CommitStaged,
    CustomNote,
}

// ── Menu ─────────────────────────────────────────────────────────────────────

pub struct MenuItem {
    pub icon: &'static str,
    pub label: &'static str,
    pub desc: &'static str,
    pub choice: Choice,
}

pub const MENU_ITEMS: &[MenuItem] = &[
    MenuItem { icon: "\u{1F680}", label: "Commit all", desc: "stage everything + commit", choice: Choice::CommitAll },
    MenuItem { icon: "\u{1F4E6}", label: "Commit staged only", desc: "spawn Claude as-is", choice: Choice::CommitStaged },
    MenuItem { icon: "\u{1F4DD}", label: "Custom note", desc: "pass message to Claude", choice: Choice::CustomNote },
];

// ── Shared Claude state ──────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub struct ClaudeState {
    pub output_lines: Arc<Mutex<Vec<String>>>,
    pub done: Arc<AtomicBool>,
    pub exit_code: Arc<Mutex<i32>>,
}

impl ClaudeState {
    pub fn new() -> Self {
        Self {
            output_lines: Arc::new(Mutex::new(Vec::new())),
            done: Arc::new(AtomicBool::new(false)),
            exit_code: Arc::new(Mutex::new(0)),
        }
    }

    pub fn is_done(&self) -> bool {
        self.done.load(Ordering::Relaxed)
    }

    pub fn get_exit_code(&self) -> i32 {
        *self.exit_code.lock().expect("exit_code lock poisoned")
    }
}

// ── App ──────────────────────────────────────────────────────────────────────

pub struct App {
    pub phase: Phase,
    pub selected: usize,
    pub choice: Choice,
    pub note: String,
    pub spinner_frame: usize,
    pub exit_code: i32,
    pub log_line: String,
    pub diff_stat: String,
    pub branch: String,
    pub staged: usize,
    pub claude: ClaudeState,
}

impl App {
    pub fn new() -> Self {
        let branch = git::branch();
        let staged = git::staged_count();

        Self {
            phase: Phase::Menu,
            selected: 1,
            choice: MENU_ITEMS[1].choice,
            note: String::new(),
            spinner_frame: 0,
            exit_code: 0,
            log_line: String::new(),
            diff_stat: String::new(),
            branch,
            staged,
            claude: ClaudeState::new(),
        }
    }

    pub fn tick(&mut self) {
        self.spinner_frame = (self.spinner_frame + 1) % BRAILLE_FRAMES.len();

        if self.phase == Phase::Running && self.claude.is_done() {
            self.exit_code = self.claude.get_exit_code();
            if self.exit_code == 0 {
                self.log_line = git::log_oneline();
                self.diff_stat = git::diff_stat();
            }
            self.phase = Phase::Summary;
        }
    }

    pub fn spawn_claude(&mut self, prompt: String, stage_all: bool) {
        self.claude.done.store(false, Ordering::Relaxed);

        let lines_ref = Arc::clone(&self.claude.output_lines);
        let done_ref = Arc::clone(&self.claude.done);
        let ec_ref = Arc::clone(&self.claude.exit_code);

        std::thread::spawn(move || {
            if stage_all {
                git::stage_all();
            }
            claude::run_subprocess(&prompt, lines_ref, done_ref, ec_ref);
        });

        self.phase = Phase::Running;
    }
}
