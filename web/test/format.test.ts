import { describe, expect, it } from 'vitest';
import { capitalise, distance, formatArea, formatBytes, formatCoord, formatDistance, formatDMS, formatDuration, formatNumber, formatOsmDate, isRecent, parseOsmDate } from '../src/ui/format.ts';

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

describe('OpenStreetMap dates', () => {
  it('parses years, months and days', () => {
    expect(parseOsmDate('1853-04-18')?.toISOString()).toBe('1853-04-18T00:00:00.000Z');
    expect(parseOsmDate('2025-10')?.toISOString()).toBe('2025-10-01T00:00:00.000Z');
    expect(parseOsmDate('~1868')?.getUTCFullYear()).toBe(1868);
    expect(parseOsmDate('early 1900s')).toBeNull();
    expect(parseOsmDate(undefined)).toBeNull();
  });

  it('formats at the tagged precision', () => {
    expect(formatOsmDate('1868')).toBe('1868');
    expect(formatOsmDate('2025-10')).toBe('Oct 2025');
    expect(formatOsmDate('2025-10-08')).toBe('8 Oct 2025');
    expect(formatOsmDate('sometime')).toBe('sometime');
  });

  it('flags openings in the last two years', () => {
    const now = new Date('2026-10-13T00:00:00Z');
    expect(isRecent('2025-10-08', now)).toBe(true);
    expect(isRecent('2024-11', now)).toBe(true);
    expect(isRecent('2024-10-01', now)).toBe(false);
    expect(isRecent('1853', now)).toBe(false);
    expect(isRecent('2027-03', now)).toBe(false); // not open yet
    expect(isRecent(undefined, now)).toBe(false);
  });
});

describe('formatBytes', () => {
  it('picks KB, MB or GB', () => {
    expect(formatBytes(200)).toBe('1 KB');
    expect(formatBytes(870_000)).toBe('850 KB');
    expect(formatBytes(1_462_520)).toBe('1.4 MB');
    expect(formatBytes(10_742_190_000)).toBe('10 GB');
  });
});
