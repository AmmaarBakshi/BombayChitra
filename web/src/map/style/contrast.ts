// WCAG contrast helpers for checking that map colours stay distinguishable on both themes.

/** Relative luminance of a #rrggbb colour (WCAG 2.x). */
export function luminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  const channel = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

/** Contrast ratio between two #rrggbb colours, 1–21. Graphics such as lines need at least 3. */
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/**
 * The tone the pipeline gives a line colour (layers/Transit.java): `dark` when too dark for the dark map, `light`
 * when too pale for the light map. Lines with a tone get a contrasting casing.
 */
export function tone(hex: string): 'dark' | 'light' | undefined {
  const l = luminance(hex);
  return l < 0.08 ? 'dark' : l > 0.45 ? 'light' : undefined;
}
