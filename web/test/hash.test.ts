import { describe, expect, it } from 'vitest';
import { encodeHash, formatRoute, hashParams, parseLatLon, parseRoute, parseSelection } from '../src/ui/hash.ts';

describe('hashParams / encodeHash', () => {
  it('reads parameters with or without the leading #', () => {
    expect(hashParams('#map=15/19.05/72.84&area=makabo').get('area')).toBe('makabo');
    expect(hashParams('sel=place:1').get('sel')).toBe('place:1');
  });

  it('keeps separators readable and round-trips', () => {
    const params = hashParams('');
    params.set('map', '15.2/19.05432/72.84021');
    params.set('sel', 'pin:19.05,72.84');
    params.set('dist', '19.1,72.8~19.0,72.9');
    params.set('q', 'a&b c');
    const text = encodeHash(params);
    expect(text).toBe('map=15.2/19.05432/72.84021&sel=pin:19.05,72.84&dist=19.1,72.8~19.0,72.9&q=a%26b+c');
    expect(hashParams(text).get('q')).toBe('a&b c');
    expect(hashParams(text).get('dist')).toBe('19.1,72.8~19.0,72.9');
  });
});

describe('parseSelection', () => {
  it('parses feature selections', () => {
    expect(parseSelection('place:311442962')).toEqual({ kind: 'place', id: 311442962 });
    expect(parseSelection('route:7972478')).toEqual({ kind: 'route', id: 7972478 });
    expect(parseSelection('building:12')).toEqual({ kind: 'building', id: 12 });
  });

  it('parses dropped pins', () => {
    expect(parseSelection('pin:19.054500,72.840500')).toEqual({ kind: 'pin', lat: 19.0545, lon: 72.8405 });
  });

  it('rejects malformed values', () => {
    for (const bad of [null, '', 'place', 'place:', 'place:abc', 'place:-1', 'unknown:5', 'pin:19', 'pin:x,y', 'pin:95,72']) {
      expect(parseSelection(bad), String(bad)).toBeNull();
    }
  });
});

describe('routes', () => {
  it('parses and formats lat,lon pairs', () => {
    expect(parseLatLon('19.1197,72.8464')).toEqual([19.1197, 72.8464]);
    expect(parseLatLon('19.1197, 72.8464')).toEqual([19.1197, 72.8464]);
    expect(parseLatLon('19.1197,')).toBeNull();
    expect(parseLatLon('200,72')).toBeNull();
  });

  it('round-trips a route', () => {
    const text = formatRoute([19.119712, 72.846433], [19.066, 72.868]);
    expect(text).toBe('19.11971,72.84643~19.06600,72.86800');
    expect(parseRoute(text)).toEqual({ from: [19.11971, 72.84643], to: [19.066, 72.868] });
  });

  it('rejects incomplete routes', () => {
    expect(parseRoute(null)).toBeNull();
    expect(parseRoute('19.1,72.8')).toBeNull();
    expect(parseRoute('19.1,72.8~')).toBeNull();
    expect(parseRoute('19.1,72.8~19.0,72.9~19.2,72.7')).toBeNull();
  });
});
