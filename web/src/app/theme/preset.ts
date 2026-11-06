import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Every color the app uses lives in this directory. The schemes below are the source: the
// PrimeVue preset is built from them, tailwind.css names them for utilities, and
// contrast.test.ts checks every pair the pages put together against WCAG AAA.

// Book cloth: the oxblood of an old library binding, the one accent.
const cloth = {
  50: '#fdf3f3',
  100: '#fbe5e6',
  200: '#f6cfd1',
  300: '#eea9ad',
  400: '#e27a81',
  500: '#d0505a',
  600: '#b73742',
  700: '#9a2a35',
  800: '#852630',
  900: '#6d232c',
  950: '#3d0f14',
}

// Paper and ink for the light scheme: warm, like the page of a book.
const paper = {
  0: '#ffffff',
  50: '#faf9f7',
  100: '#f3f1ed',
  200: '#e6e2db',
  300: '#d3cdc3',
  400: '#a8a196',
  500: '#7c766c',
  600: '#5e5951',
  700: '#46423c',
  800: '#2f2c28',
  900: '#1e1c19',
  950: '#12110f',
}

// Slate for the dark scheme: cool, a reading lamp's room rather than the light one inverted.
const slate = {
  0: '#ffffff',
  50: '#f6f7f9',
  100: '#eceef2',
  200: '#d5d9e0',
  300: '#b1b8c4',
  400: '#8791a2',
  500: '#687387',
  600: '#535d6f',
  700: '#444c5b',
  800: '#2b313b',
  900: '#1c2028',
  950: '#111419',
}

/** The colors of one scheme, named for what they do. */
export interface Scheme {
  /** Behind everything. */
  page: string
  /** A panel, a dialog, the bars. */
  panel: string
  /** Inside a field. */
  sunken: string
  /** A secondary button, a hovered row. */
  raised: string
  /** Decorative lines between parts; never the only edge of a control. */
  hairline: string
  /** The edge of a control: fields, secondary buttons, switches. */
  outline: string
  ink: string
  inkMuted: string
  accent: string
  accentHover: string
  /** Text and icons on the accent. */
  accentContrast: string
  /** A selected chip, row, or page. */
  highlight: string
  highlightInk: string
  danger: string
  dangerHover: string
  dangerContrast: string
  success: string
}

export const schemes: { light: Scheme; dark: Scheme } = {
  light: {
    page: paper[100],
    panel: paper[0],
    sunken: paper[0],
    raised: paper[100],
    hairline: paper[300],
    outline: paper[500],
    ink: paper[900],
    inkMuted: '#4a453f',
    accent: cloth[800],
    accentHover: cloth[900],
    accentContrast: '#ffffff',
    highlight: cloth[100],
    highlightInk: cloth[900],
    danger: '#8a1c1c',
    dangerHover: '#701616',
    dangerContrast: '#ffffff',
    success: '#1d5c33',
  },
  dark: {
    page: slate[950],
    panel: slate[900],
    sunken: slate[950],
    raised: slate[800],
    hairline: slate[700],
    outline: slate[400],
    ink: slate[50],
    inkMuted: '#bcc2cc',
    accent: cloth[200],
    accentHover: cloth[100],
    accentContrast: cloth[950],
    highlight: '#40262c',
    highlightInk: cloth[100],
    danger: '#fbb4b4',
    dangerHover: '#fecaca',
    dangerContrast: cloth[950],
    success: '#9ae6b4',
  },
}

function semantic(s: Scheme, surface: Record<number, string>) {
  return {
    surface,
    primary: {
      color: s.accent,
      contrastColor: s.accentContrast,
      hoverColor: s.accentHover,
      activeColor: s.accentHover,
    },
    highlight: {
      background: s.highlight,
      focusBackground: s.highlight,
      color: s.highlightInk,
      focusColor: s.highlightInk,
    },
    text: { color: s.ink, hoverColor: s.ink, mutedColor: s.inkMuted, hoverMutedColor: s.ink },
    content: {
      background: s.panel,
      hoverBackground: s.raised,
      borderColor: s.hairline,
      color: s.ink,
      hoverColor: s.ink,
    },
    formField: {
      background: s.sunken,
      disabledBackground: s.raised,
      filledBackground: s.raised,
      filledHoverBackground: s.raised,
      filledFocusBackground: s.sunken,
      borderColor: s.outline,
      hoverBorderColor: s.inkMuted,
      focusBorderColor: s.accent,
      invalidBorderColor: s.danger,
      color: s.ink,
      disabledColor: s.inkMuted,
      placeholderColor: s.inkMuted,
      invalidPlaceholderColor: s.danger,
      floatLabelColor: s.inkMuted,
      floatLabelFocusColor: s.accent,
      floatLabelActiveColor: s.inkMuted,
      floatLabelInvalidColor: s.danger,
      iconColor: s.inkMuted,
    },
    // The app's own tokens, which PrimeVue turns into --p-page-background and --p-status-*.
    page: { background: s.page },
    status: { danger: s.danger, success: s.success },
  }
}

// PrimeVue's components draw some of their own colors from mid-grey steps of the surface,
// which fall short of AAA on these surfaces; each is pointed at a scheme color instead.
function buttons(s: Scheme) {
  return {
    root: {
      secondary: {
        background: s.raised,
        hoverBackground: s.hairline,
        activeBackground: s.hairline,
        borderColor: s.outline,
        hoverBorderColor: s.inkMuted,
        activeBorderColor: s.inkMuted,
        color: s.ink,
        hoverColor: s.ink,
        activeColor: s.ink,
        focusRing: { color: s.accent, shadow: 'none' },
      },
      danger: {
        background: s.danger,
        hoverBackground: s.dangerHover,
        activeBackground: s.dangerHover,
        borderColor: s.danger,
        hoverBorderColor: s.dangerHover,
        activeBorderColor: s.dangerHover,
        color: s.dangerContrast,
        hoverColor: s.dangerContrast,
        activeColor: s.dangerContrast,
        focusRing: { color: s.danger, shadow: 'none' },
      },
    },
    text: {
      primary: { hoverBackground: s.raised, activeBackground: s.raised, color: s.accent },
      secondary: { hoverBackground: s.raised, activeBackground: s.raised, color: s.inkMuted },
      danger: { hoverBackground: s.raised, activeBackground: s.raised, color: s.danger },
    },
  }
}

// A choice in a set of buttons: the chosen one is filled with the accent, so the state is
// told by more than a shade of grey.
function toggleButtons(s: Scheme) {
  return {
    root: {
      background: s.raised,
      checkedBackground: s.raised,
      hoverBackground: s.raised,
      borderColor: s.outline,
      checkedBorderColor: s.outline,
      color: s.inkMuted,
      hoverColor: s.ink,
      checkedColor: s.accentContrast,
    },
    content: { checkedBackground: s.accent },
    icon: { color: s.inkMuted, hoverColor: s.ink, checkedColor: s.accentContrast },
  }
}

// A switch is drawn at the target size: the default is a third of it.
function switches(s: Scheme) {
  return {
    root: {
      background: s.outline,
      hoverBackground: s.inkMuted,
      checkedBackground: s.accent,
      checkedHoverBackground: s.accentHover,
    },
    handle: {
      background: s.panel,
      hoverBackground: s.panel,
      checkedBackground: s.panel,
      checkedHoverBackground: s.panel,
    },
  }
}

// A toast sits on the panel color with a colored edge, so its words keep the panel's contrast.
function toasts(s: Scheme) {
  const tone = (color: string) => ({
    background: s.panel,
    borderColor: color,
    color,
    detailColor: s.ink,
    closeButton: { hoverBackground: s.raised, focusRing: { color, shadow: 'none' } },
  })
  return { success: tone(s.success), error: tone(s.danger), info: tone(s.ink), warn: tone(s.ink) }
}

export const preset = definePreset(Aura, {
  semantic: {
    primary: cloth,
    focusRing: {
      width: '2px',
      style: 'solid',
      color: '{primary.color}',
      offset: '2px',
      shadow: 'none',
    },
    colorScheme: {
      light: semantic(schemes.light, paper),
      dark: semantic(schemes.dark, slate),
    },
  },
  components: {
    button: { colorScheme: { light: buttons(schemes.light), dark: buttons(schemes.dark) } },
    togglebutton: {
      colorScheme: { light: toggleButtons(schemes.light), dark: toggleButtons(schemes.dark) },
    },
    toggleswitch: {
      root: { width: '4.5rem', height: '2.75rem', gap: '0.25rem' },
      handle: { size: '2.25rem' },
      colorScheme: { light: switches(schemes.light), dark: switches(schemes.dark) },
    },
    // A toast's close button at the target size: the default is 28 pixels.
    toast: {
      closeButton: { width: '2.75rem', height: '2.75rem' },
      colorScheme: { light: toasts(schemes.light), dark: toasts(schemes.dark) },
    },
  },
})
