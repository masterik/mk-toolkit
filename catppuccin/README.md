# catppuccin

Soothing pastel [Catppuccin](https://catppuccin.com) colors for the Claude desktop and mobile apps,
in the spirit of the Catppuccin VS Code theme: **Latte** in light mode, **Frappé**, **Macchiato** or
**Mocha** (default) in dark mode, with any of the 14 Catppuccin accents (default mauve) driving links,
buttons, focus and code keywords. Strings are green, numbers peach, errors red, warnings yellow; git
added/removed/modified/merged use green/red/yellow/mauve, as in the VS Code port.

The conversation sits on the flavor's `base` (the VS Code editor color), drawn as the backdrop behind it;
panels and menus step up toward `surface0`, so they read lighter than the chat.

## Install

```
/plugin marketplace add masterik/mk-toolkit
/plugin install catppuccin@masterik
```

## Use

- `/catppuccin macchiato` — dark flavor
- `/catppuccin blue` — accent
- `/catppuccin frappe lavender` — both
- `/catppuccin list` · `/catppuccin reset` (back to the config menu's values)

The config menu has *Dark flavor* and *Accent* rows too. The terminal draws no theme.

Palette: [catppuccin/palette](https://github.com/catppuccin/palette) (MIT).
