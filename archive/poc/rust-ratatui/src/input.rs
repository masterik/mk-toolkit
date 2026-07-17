use crossterm::event::{self, Event, KeyCode, KeyEventKind, KeyModifiers};
use std::time::Duration;

use crate::app::{App, Choice, Phase, MENU_ITEMS, POLL_INTERVAL_MS};

/// Result of processing one tick of input.
pub enum InputResult {
    /// No input event occurred (timeout).
    None,
    /// Continue the main loop.
    Continue,
    /// Quit the application.
    Quit,
}

/// Poll for keyboard input and update app state accordingly.
pub fn handle(app: &mut App) -> Result<InputResult, Box<dyn std::error::Error>> {
    if !event::poll(Duration::from_millis(POLL_INTERVAL_MS))? {
        return Ok(InputResult::None);
    }

    let Event::Key(key) = event::read()? else {
        return Ok(InputResult::Continue);
    };

    if key.kind != KeyEventKind::Press {
        return Ok(InputResult::Continue);
    }

    match app.phase {
        Phase::Menu => handle_menu(app, key.code),
        Phase::NoteInput => handle_note_input(app, key.code),
        Phase::Running => handle_running(app, key.code, key.modifiers),
        Phase::Summary => handle_summary(app, key.code),
    }
}

fn handle_menu(app: &mut App, code: KeyCode) -> Result<InputResult, Box<dyn std::error::Error>> {
    match code {
        KeyCode::Up | KeyCode::Char('k') => {
            app.selected = app.selected.saturating_sub(1);
        }
        KeyCode::Down | KeyCode::Char('j') => {
            if app.selected < MENU_ITEMS.len() - 1 {
                app.selected += 1;
            }
        }
        KeyCode::Enter => {
            app.choice = MENU_ITEMS[app.selected].choice;
            if app.choice == Choice::CustomNote {
                app.phase = Phase::NoteInput;
            } else {
                let prompt = "/commit".to_string();
                let stage_all = app.choice == Choice::CommitAll;
                app.spawn_claude(prompt, stage_all);
            }
        }
        KeyCode::Char('q') | KeyCode::Esc => return Ok(InputResult::Quit),
        _ => {}
    }
    Ok(InputResult::Continue)
}

fn handle_note_input(app: &mut App, code: KeyCode) -> Result<InputResult, Box<dyn std::error::Error>> {
    match code {
        KeyCode::Char(c) => app.note.push(c),
        KeyCode::Backspace => { app.note.pop(); }
        KeyCode::Enter => {
            let prompt = if app.note.is_empty() {
                "/commit".to_string()
            } else {
                format!("/commit {}", app.note)
            };
            app.spawn_claude(prompt, false);
        }
        KeyCode::Esc => {
            app.note.clear();
            app.phase = Phase::Menu;
        }
        _ => {}
    }
    Ok(InputResult::Continue)
}

fn handle_running(
    _app: &mut App,
    code: KeyCode,
    modifiers: KeyModifiers,
) -> Result<InputResult, Box<dyn std::error::Error>> {
    if code == KeyCode::Char('c') && modifiers.contains(KeyModifiers::CONTROL) {
        return Ok(InputResult::Quit);
    }
    Ok(InputResult::Continue)
}

fn handle_summary(_app: &mut App, code: KeyCode) -> Result<InputResult, Box<dyn std::error::Error>> {
    match code {
        KeyCode::Char('q') | KeyCode::Enter | KeyCode::Esc => Ok(InputResult::Quit),
        _ => Ok(InputResult::Continue),
    }
}
