// WCAG's measure of how far apart two colours read, for the theme's pairs and for a spine's
// lettering on its cover's colour.

/** The channels of a colour written #rrggbb, each 0 to 255. */
export function channels(hex: string): number[] {
  return (hex.replace('#', '').match(/../g) ?? []).map((x) => parseInt(x, 16))
}

/** WCAG's relative luminance of a colour written #rrggbb. */
export function luminance(hex: string): number {
  const [r = 0, g = 0, b = 0] = channels(hex).map((c) => {
    const v = c / 255
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** WCAG's contrast ratio between two colours, the lighter's over the darker's. */
export function contrast(a: string, b: string): number {
  const [hi = 0, lo = 0] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
