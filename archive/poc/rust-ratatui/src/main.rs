mod app;
mod claude;
mod git;
mod input;
mod ui;

use std::io;
use std::time::Instant;

use crossterm::execute;
use crossterm::terminal::{
    disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen,
};
use ratatui::prelude::*;

use crate::app::App;
use crate::input::InputResult;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let start = Instant::now();

    let mut app = App::new();

    eprintln!("[fz] ready in {}ms", start.elapsed().as_millis());

    // Setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Main loop
    loop {
        terminal.draw(|frame| ui::draw(frame, &app))?;
        app.tick();

        match input::handle(&mut app)? {
            InputResult::Quit => break,
            InputResult::Continue | InputResult::None => {}
        }
    }

    // Restore terminal
    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;

    std::process::exit(app.exit_code);
}
