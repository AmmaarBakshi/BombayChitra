import { describe, expect, it } from 'vitest';
import { contrast, luminance, tone } from '../src/map/style/contrast.ts';
import { palettes } from '../src/map/style/palette.ts';

/** Every line colour in the current dataset (SELECT DISTINCT colour FROM transit_lines). */
const LINE_COLOURS = ['#00a3b4', '#2b6cd4', '#2e9e4f', '#303888', '#4b0082', '#5c6ac4', '#77248b', '#b04fa0', '#b82880', '#e03131', '#e64980', '#ebcf1f', '#f0d808', '#f2b705', '#f76b15', '#ff0000'];

describe('contrast', () => {
  it('computes WCAG ratios', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrast('#777777', '#777777')).toBe(1);
    expect(luminance('#ffffff')).toBeCloseTo(1, 5);
  });

  it.each(['light', 'dark'] as const)('every metro line stands out on the %s map', (theme) => {
    const P = palettes[theme];
    for (const colour of LINE_COLOURS) {
      const casing = tone(colour) === (theme === 'dark' ? 'dark' : 'light') ? P.transitToneCasing : P.transitCasing;
      // Readable when the line contrasts 3:1 with the map or with its casing.
      const best = Math.max(contrast(colour, P.land), contrast(colour, casing));
      expect(best, `${colour} on the ${theme} map`).toBeGreaterThanOrEqual(2.95);
    }
  });

  it('agrees with the pipeline on tones', () => {
    expect(tone('#4b0082')).toBe('dark');
    expect(tone('#f2b705')).toBe('light');
    expect(tone('#e03131')).toBeUndefined();
  });
});
