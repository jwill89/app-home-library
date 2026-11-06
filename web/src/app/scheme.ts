import { watch } from 'vue'
import { useRemembered } from './remember'

// Light or dark: as the device is set, or as the owner chose on this device. The whole theme
// follows one class on the page's root, the one PrimeVue's dark scheme is keyed to (main.ts).

export const schemes = [
  { value: 'device', label: 'Match this device' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
] as const

export type Scheme = (typeof schemes)[number]['value']

/** The class the dark scheme answers to, on the page's root. */
export const darkClass = 'scheme-dark'

/** The scheme chosen on this device: the device's own until the owner picks another. */
export const scheme = useRemembered<Scheme>(
  'hl.scheme',
  schemes.map((s) => s.value),
  'device',
)

const deviceDark = window.matchMedia('(prefers-color-scheme: dark)')

// The browser's bar takes the color index.html gives each scheme; a choice paints both with its own.
const bars = [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')]
const barColors = new Map(bars.map((m) => [m.media.includes('dark'), m.content]))

function apply(choice: Scheme) {
  const dark = choice === 'dark' || (choice === 'device' && deviceDark.matches)
  document.documentElement.classList.toggle(darkClass, dark)
  for (const bar of bars) {
    const own = bar.media.includes('dark')
    bar.content = barColors.get(choice === 'device' ? own : choice === 'dark') ?? bar.content
  }
}

/** Puts the chosen scheme on the page, and keeps it there as the choice or the device changes. */
export function startScheme() {
  watch(scheme, apply, { immediate: true })
  deviceDark.addEventListener('change', () => apply(scheme.value))
}
