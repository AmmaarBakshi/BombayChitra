import { describe, expect, it } from 'vitest';
import type { RouteStep } from '../src/api/types.ts';
import { detectLanguage, type Key, STRINGS, stepText, t } from '../src/i18n.ts';

const step = (s: Partial<RouteStep>): RouteStep => ({ type: 'turn', distance_m: 100, duration_s: 20, location: [72.8, 19], bearing: 0, instruction: 'Turn left onto Linking Road', ...s });

describe('i18n', () => {
  it('translates and fills placeholders', () => {
    expect(t('tab.distance', {}, 'en')).toBe('Distance');
    expect(t('tab.distance', {}, 'mr')).toBe('अंतर');
    expect(t('tab.distance', {}, 'hi')).toBe('दूरी');
    expect(t('step.depart', { dir: 'north' }, 'en')).toBe('Head north');
  });

  it('covers every English string in Marathi and Hindi', () => {
    const keys = Object.keys(STRINGS.en) as Key[];
    for (const lang of ['mr', 'hi'] as const) {
      const missing = keys.filter((k) => !STRINGS[lang][k]);
      expect(missing, `${lang} is missing strings`).toEqual([]);
    }
  });

  it('picks the saved language, then the browser language, then English', () => {
    expect(detectLanguage('hi', ['mr-IN'])).toBe('hi');
    expect(detectLanguage(null, ['mr-IN', 'en'])).toBe('mr');
    expect(detectLanguage(null, ['en-IN', 'hi'])).toBe('en');
    expect(detectLanguage(null, ['fr-FR'])).toBe('en');
    expect(detectLanguage('xx', [])).toBe('en');
  });

  it('builds directions from step codes', () => {
    const turn = step({ modifier: 'left', name: 'Linking Road' });
    expect(stepText(turn, 'en')).toBe('Turn left onto Linking Road');
    expect(stepText(turn, 'mr')).toBe('डावीकडे वळा — Linking Road');
    expect(stepText(turn, 'hi')).toBe('बाएँ मुड़ें — Linking Road');
    expect(stepText(step({ type: 'depart', bearing: 92, instruction: 'Head east' }), 'hi')).toBe('पूर्व की ओर चलें');
    expect(stepText(step({ type: 'arrive' }), 'mr')).toBe('आपण पोहोचलात');
    expect(stepText(step({ type: 'ramp', modifier: 'slight_left' }), 'mr')).toBe('डावीकडील रॅम्प घ्या');
    expect(stepText(step({ type: 'continue', modifier: 'straight' }), 'hi')).toBe('सीधे चलते रहें');
  });
});
