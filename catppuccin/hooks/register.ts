import type { EngineInterface, Register, ThemeSetArgs } from 'claude-code'

import { makeColors } from './make-colors.js'
import { ACCENTS, DARK_FLAVORS, FLAVORS, mix } from './palette'
import type { Accent, DarkFlavor, Palette } from './palette'

const [S1, S2, S3] = [0.5, 0.8, 0.9]

const LABELS: Record<DarkFlavor, string> = { frappe: 'Frappé', macchiato: 'Macchiato', mocha: 'Mocha' }

/** The whole look for Latte (light) plus one dark flavor, in one accent. */
export function buildLook(flavor: DarkFlavor, accent: Accent): { look: ThemeSetArgs; problems: string[] } {
  const latte: Palette = FLAVORS.latte
  const dark: Palette = FLAVORS[flavor]

  const mode = (p: Palette, isDark: boolean) => ({
    gray: {
      anchors: isDark
        ? {
            // The conversation sits on `base`, as VS Code's editor does; panels and menus
            // step up toward `surface0` so they read lighter than the chat.
            'surface-0': p.base,
            'surface-1': mix(p.base, p.surface0, S1),
            'surface-2': mix(p.base, p.surface0, S2),
            'surface-3': mix(p.base, p.surface0, S3),
            'text-primary': p.text,
            'text-secondary': p.subtext1,
            'text-muted': p.subtext0,
          }
        : {
            // Light mode wants cards lighter than the page: `base` page, cards toward white.
            'surface-0': p.base,
            'surface-1': mix(p.base, '#ffffff', 0.4),
            'surface-2': mix(p.base, '#ffffff', 0.7),
            'surface-3': mix(p.base, '#ffffff', 0.7),
            'text-primary': p.text,
            'text-secondary': p.subtext1,
            'text-muted': p.subtext0,
          },
    },
    // As the VS Code port colors code: keywords mauve-ish accent, strings green, numbers peach.
    // Dark warning fills one stop deeper than stock, so text reads on Frappé's comment highlight.
    roles: { accent: p[accent], danger: p.red, success: p.green, warning: isDark ? { seed: p.yellow, fillShade: 300 } : p.yellow, pro: p.peach },
    git: {
      added: p.green,
      removed: p.red,
      modified: p.yellow,
      merged: p.mauve,
      closed: p.red,
      conflicting: p.peach,
      draft: p.overlay2,
    },
  })

  const { theme, problems } = makeColors({
    label: `Catppuccin Latte / ${LABELS[flavor]} (${accent})`,
    id: `catppuccin-${flavor}-${accent}`,
    light: mode(latte, false),
    dark: mode(dark, true),
    codeTheme: { light: 'catppuccin-latte', dark: `catppuccin-${flavor}` },
  })

  // The person's bubble: Catppuccin `surface0`, as VS Code draws selections and widgets.
  const inputs = theme.inputs as Record<'light' | 'dark', Record<string, string> | undefined>
  if (inputs.light) {
    inputs.light['bg-user-message'] = latte.surface0
    inputs.light['text-user-message'] = latte.text
  }
  if (inputs.dark) {
    inputs.dark['bg-user-message'] = dark.surface0
    inputs.dark['text-user-message'] = dark.text
  }

  // The ground behind the conversation is the page color (`base` dark, Latte `base` light), so the
  // chat column reads as the darkest surface and the frosted panels over it as lighter.
  return { look: { colors: theme, backdrop: { color: 'surface-0' } }, problems }
}

function isFlavor(v: unknown): v is DarkFlavor {
  return typeof v === 'string' && (DARK_FLAVORS as readonly string[]).includes(v)
}
function isAccent(v: unknown): v is Accent {
  return typeof v === 'string' && (ACCENTS as readonly string[]).includes(v)
}

type Choice = { flavor: DarkFlavor; accent: Accent }

/** The /catppuccin choice wins over the config menu's until reset. */
async function current($: EngineInterface, fallback: Choice): Promise<Choice> {
  const flavor = await $.store.get('flavor')
  const accent = await $.store.get('accent')
  return {
    flavor: isFlavor(flavor) ? flavor : fallback.flavor,
    accent: isAccent(accent) ? accent : fallback.accent,
  }
}

async function apply($: EngineInterface, choice: Choice): Promise<void> {
  const { look, problems } = buildLook(choice.flavor, choice.accent)
  const unpainted = problems.find(line => line.startsWith("Desktop won't paint"))
  if (unpainted !== undefined) {
    $.ui.toast(unpainted)
  }
  let set
  try {
    set = await $.theme.set(look)
  } catch (error) {
    // A build that takes no backdrop: the colors alone.
    $.ui.log(String(error))
    set = await $.theme.set({ colors: look.colors })
  }
  if (!set.isDrawn) {
    $.ui.status(set.reason)
  }
}

export const register: Register = (on, options) => {
  const fromOptions: Choice = {
    flavor: isFlavor(options.flavor) ? options.flavor : ('mocha' as DarkFlavor),
    accent: isAccent(options.accent) ? options.accent : ('mauve' as Accent),
  }

  on('session.start', async ($, e, next) => {
    await $.command.register({
      name: 'catppuccin',
      description: 'Catppuccin: /catppuccin [frappe|macchiato|mocha] [accent] · reset · list',
    })
    await apply($, await current($, fromOptions))
    return next(e)
  })

  on('command.run', { command: 'catppuccin' }, async ($, e) => {
    const words = e.args.toLowerCase().split(/\s+/).filter(Boolean)

    if (words[0] === 'list') {
      return { text: `Flavors: ${DARK_FLAVORS.join(', ')}. Accents: ${ACCENTS.join(', ')}.` }
    }
    if (words[0] === 'reset') {
      await $.store.delete('flavor')
      await $.store.delete('accent')
    }

    const bad = words.filter(w => w !== 'reset' && !isFlavor(w) && !isAccent(w))
    if (bad.length > 0) {
      return { text: `Unknown: ${bad.join(', ')}. Try /catppuccin list.` }
    }
    const flavor = words.find(isFlavor)
    const accent = words.find(isAccent)
    if (flavor !== undefined) await $.store.set('flavor', flavor)
    if (accent !== undefined) await $.store.set('accent', accent)

    const now = await current($, fromOptions)
    await apply($, now)
    return { text: `Catppuccin: Latte by day, ${LABELS[now.flavor]} by night, ${now.accent} accent.` }
  })
}
