use ratatui::prelude::*;
use ratatui::widgets::*;

use crate::app::{App, Phase, BRAILLE_FRAMES, MENU_ITEMS};

// ── Layout constants ─────────────────────────────────────────────────────────

const HEADER_HEIGHT: u16 = 4;
const GAP_HEIGHT: u16 = 1;
const HELP_HEIGHT: u16 = 1;
const MIN_CONTENT_HEIGHT: u16 = 5;

// ── Public entry point ───────────────────────────────────────────────────────

pub fn draw(frame: &mut Frame, app: &App) {
    let area = frame.area();

    let layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(HEADER_HEIGHT),
            Constraint::Length(GAP_HEIGHT),
            Constraint::Min(MIN_CONTENT_HEIGHT),
            Constraint::Length(GAP_HEIGHT),
            Constraint::Length(HELP_HEIGHT),
        ])
        .split(area);

    draw_header(layout[0], frame.buffer_mut(), &app.branch, app.staged);

    match app.phase {
        Phase::Menu => draw_menu_phase(layout[2], layout[4], frame, app.selected),
        Phase::NoteInput => draw_note_phase(layout[2], layout[4], frame, &app.note),
        Phase::Running => draw_running_phase(layout[2], layout[4], frame, app),
        Phase::Summary => draw_summary_phase(layout[2], layout[4], frame, app),
    }
}

// ── Header ───────────────────────────────────────────────────────────────────

fn draw_header(area: Rect, buf: &mut Buffer, branch: &str, staged: usize) {
    let title = Line::from(vec![
        Span::styled("\u{26A1} ForgeZ", Style::default().bold()),
        Span::raw(" \u{2014} commit"),
    ]);
    let info = Line::from(vec![
        Span::raw("\u{1F4CD} "),
        Span::styled(branch, Style::default().fg(Color::Cyan).bold()),
        Span::raw(format!(" \u{00B7} {} files staged", staged)),
    ]);

    let block = Block::default()
        .borders(Borders::ALL)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(Color::Cyan));

    Paragraph::new(vec![title, info]).block(block).render(area, buf);
}

// ── Menu phase ───────────────────────────────────────────────────────────────

fn draw_menu_phase(content_area: Rect, help_area: Rect, frame: &mut Frame, selected: usize) {
    let content_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Length(3),
        ])
        .split(content_area);

    Paragraph::new(Span::styled(
        "How would you like to commit?",
        Style::default().bold(),
    ))
    .render(content_layout[0], frame.buffer_mut());

    draw_menu_list(content_layout[2], frame.buffer_mut(), selected);
    draw_help(help_area, frame.buffer_mut(), &[
        ("\u{2191}/\u{2193}", "navigate"),
        ("enter", "select"),
        ("q", "quit"),
    ]);
}

fn draw_menu_list(area: Rect, buf: &mut Buffer, selected: usize) {
    let items: Vec<ListItem> = MENU_ITEMS
        .iter()
        .enumerate()
        .map(|(i, item)| {
            let is_selected = i == selected;
            let indicator = if is_selected { "\u{25CF}" } else { "\u{25CB}" };
            let style = if is_selected {
                Style::default().fg(Color::Cyan).bold()
            } else {
                Style::default()
            };
            let indicator_style = if is_selected {
                style
            } else {
                Style::default().fg(Color::DarkGray)
            };

            let line = Line::from(vec![
                Span::styled(format!(" {} ", indicator), indicator_style),
                Span::raw(format!("{} ", item.icon)),
                Span::styled(item.label, style),
                Span::styled(
                    format!(" ({})", item.desc),
                    Style::default().fg(Color::DarkGray).italic(),
                ),
            ]);
            ListItem::new(line)
        })
        .collect();

    Widget::render(List::new(items), area, buf);
}

// ── Note input phase ─────────────────────────────────────────────────────────

fn draw_note_phase(content_area: Rect, help_area: Rect, frame: &mut Frame, note: &str) {
    let content_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Length(3),
        ])
        .split(content_area);

    Paragraph::new(Span::styled(
        "Enter commit note:",
        Style::default().bold(),
    ))
    .render(content_layout[0], frame.buffer_mut());

    let input_block = Block::default()
        .borders(Borders::ALL)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(Color::Cyan))
        .title(" note ");

    let display_text = if note.is_empty() {
        Span::styled(
            "Describe your changes...",
            Style::default().fg(Color::DarkGray).italic(),
        )
    } else {
        Span::raw(note)
    };

    Paragraph::new(display_text)
        .block(input_block)
        .render(content_layout[2], frame.buffer_mut());

    let cursor_x = content_layout[2].x + 1 + note.len() as u16;
    let cursor_y = content_layout[2].y + 1;
    frame.set_cursor_position(Position::new(cursor_x, cursor_y));

    draw_help(help_area, frame.buffer_mut(), &[
        ("enter", "submit"),
        ("esc", "back"),
        ("ctrl+c", "quit"),
    ]);
}

// ── Running phase ────────────────────────────────────────────────────────────

fn draw_running_phase(content_area: Rect, help_area: Rect, frame: &mut Frame, app: &App) {
    let content_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Min(3),
        ])
        .split(content_area);

    // Spinner line
    let frame_char = BRAILLE_FRAMES[app.spinner_frame % BRAILLE_FRAMES.len()];
    let spinner_line = Line::from(vec![
        Span::styled(
            format!("{} ", frame_char),
            Style::default().fg(Color::Cyan).bold(),
        ),
        Span::raw("Running Claude..."),
    ]);
    Paragraph::new(spinner_line).render(content_layout[0], frame.buffer_mut());

    // Output box
    draw_output_box(content_layout[2], frame.buffer_mut(), &app.claude.output_lines);

    draw_help(help_area, frame.buffer_mut(), &[("ctrl+c", "abort")]);
}

// ── Summary phase ────────────────────────────────────────────────────────────

fn draw_summary_phase(content_area: Rect, help_area: Rect, frame: &mut Frame, app: &App) {
    let content_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Min(4),
            Constraint::Length(1),
            Constraint::Length(6),
        ])
        .split(content_area);

    // Output box
    {
        let lines = app.claude.output_lines.lock().expect("output_lines lock poisoned");
        if !lines.is_empty() {
            let owned: Vec<String> = lines.clone();
            drop(lines);
            let text: Vec<Line> = owned
                .iter()
                .map(|l| Line::from(Span::styled(l.as_str(), Style::default().fg(Color::DarkGray))))
                .collect();

            let visible_height = content_layout[0].height.saturating_sub(2) as usize;
            let scroll = text.len().saturating_sub(visible_height) as u16;

            let output_block = Block::default()
                .borders(Borders::ALL)
                .border_type(BorderType::Rounded)
                .border_style(Style::default().fg(Color::DarkGray))
                .title(Span::styled(
                    " claude output ",
                    Style::default().fg(Color::DarkGray).italic(),
                ));

            Paragraph::new(text)
                .block(output_block)
                .scroll((scroll, 0))
                .render(content_layout[0], frame.buffer_mut());
        }
    }

    // Summary box
    if app.exit_code == 0 {
        let summary_text = vec![
            Line::from(Span::styled(
                "\u{2705} Committed!",
                Style::default().fg(Color::Green).bold(),
            )),
            Line::from(""),
            Line::from(Span::styled(&app.log_line, Style::default().bold())),
            Line::from(Span::styled(
                &app.diff_stat,
                Style::default().fg(Color::DarkGray),
            )),
        ];

        let block = Block::default()
            .borders(Borders::ALL)
            .border_type(BorderType::Rounded)
            .border_style(Style::default().fg(Color::Green))
            .title(Span::styled(
                " summary ",
                Style::default().fg(Color::Green).bold(),
            ));

        Paragraph::new(summary_text)
            .block(block)
            .render(content_layout[2], frame.buffer_mut());
    } else {
        let block = Block::default()
            .borders(Borders::ALL)
            .border_type(BorderType::Rounded)
            .border_style(Style::default().fg(Color::Red))
            .title(Span::styled(
                " error ",
                Style::default().fg(Color::Red).bold(),
            ));

        Paragraph::new(Line::from(Span::styled(
            format!("\u{274C} Claude exited with code {}", app.exit_code),
            Style::default().fg(Color::Red).bold(),
        )))
        .block(block)
        .render(content_layout[2], frame.buffer_mut());
    }

    draw_help(help_area, frame.buffer_mut(), &[
        ("q", "quit"),
        ("enter", "exit"),
    ]);
}

// ── Shared widgets ───────────────────────────────────────────────────────────

fn draw_output_box(area: Rect, buf: &mut Buffer, output_lines: &std::sync::Mutex<Vec<String>>) {
    let lines = output_lines.lock().expect("output_lines lock poisoned");
    let text: Vec<Line> = lines
        .iter()
        .map(|l| Line::from(Span::styled(l.as_str(), Style::default().fg(Color::DarkGray))))
        .collect();

    let visible_height = area.height.saturating_sub(2) as usize;
    let scroll = text.len().saturating_sub(visible_height) as u16;

    let output_block = Block::default()
        .borders(Borders::ALL)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(Color::DarkGray))
        .title(Span::styled(
            " claude output ",
            Style::default().fg(Color::DarkGray).italic(),
        ));

    Paragraph::new(text)
        .block(output_block)
        .scroll((scroll, 0))
        .render(area, buf);
}

fn draw_help(area: Rect, buf: &mut Buffer, keys: &[(&str, &str)]) {
    let spans: Vec<Span> = keys
        .iter()
        .enumerate()
        .flat_map(|(i, (key, desc))| {
            let mut v = vec![
                Span::styled(*key, Style::default().fg(Color::Cyan).bold()),
                Span::styled(format!(" {}", desc), Style::default().fg(Color::DarkGray)),
            ];
            if i < keys.len() - 1 {
                v.push(Span::raw("  "));
            }
            v
        })
        .collect();

    Paragraph::new(Line::from(spans)).render(area, buf);
}
