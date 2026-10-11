import { describe, expect, it } from 'vitest';
import { capitalise, distance, formatArea, formatCoord, formatDistance, formatDMS, formatDuration, formatNumber } from '../src/ui/format.ts';

describe('formatDistance', () => {
  it('uses metres below a kilometre', () => {
    expect(formatDistance(0)).toBe('0 m');
    expect(formatDistance(349.6)).toBe('350 m');
    expect(formatDistance(999)).toBe('999 m');
  });
  it('uses kilometres with one decimal above', () => {
    expect(formatDistance(1000)).toBe('1 km');
    expect(formatDistance(4040)).toBe('4 km');
    expect(formatDistance(18_237)).toBe('18.2 km');
  });
});

describe('formatDuration', () => {
  it('rounds to whole minutes, never below one', () => {
    expect(formatDuration(0)).toBe('1 min');
    expect(formatDuration(89)).toBe('1 min');
    expect(formatDuration(91)).toBe('2 min');
    expect(formatDuration(59 * 60)).toBe('59 min');
  });
  it('switches to hours', () => {
    expect(formatDuration(3600)).toBe('1 h');
    expect(formatDuration(3600 + 5 * 60)).toBe('1 h 5 min');
    expect(formatDuration(2.5 * 3600)).toBe('2 h 30 min');
  });
});

describe('formatArea', () => {
  it('picks m², hectares or km²', () => {
    expect(formatArea(450)).toBe('450 m²');
    expect(formatArea(25_000)).toBe('2.5 ha');
    expect(formatArea(68_400_000)).toBe('68.4 km²');
  });
});

describe('numbers', () => {
  it('groups digits the Indian way', () => {
    expect(formatNumber(154607)).toBe('1,54,607');
    expect(formatNumber(2_100_000)).toBe('21,00,000');
  });
});

describe('coordinates', () => {
  it('prints latitude first', () => {
    expect(formatCoord(72.8402, 19.0544, 4)).toBe('19.0544, 72.8402');
  });
  it('prints degrees, minutes and seconds', () => {
    expect(formatDMS(72.8777, 19.076)).toBe('19°4′33.6″N 72°52′39.7″E');
    expect(formatDMS(-0.5, -0.25)).toBe('0°15′0.0″S 0°30′0.0″W');
  });
});

describe('distance', () => {
  it('measures great-circle distance in metres', () => {
    // Churchgate to Virar is about 57 km as the crow flies.
    expect(distance([72.8258, 18.9353], [72.8114, 19.4559])).toBeGreaterThan(57_000);
    expect(distance([72.8258, 18.9353], [72.8114, 19.4559])).toBeLessThan(58_500);
    expect(distance([72.8, 19], [72.8, 19])).toBe(0);
  });
});

describe('capitalise', () => {
  it('capitalises and replaces underscores', () => {
    expect(capitalise('light_rail')).toBe('Light rail');
    expect(capitalise('')).toBe('');
  });
});
