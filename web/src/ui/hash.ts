// Shareable state lives in the URL hash next to MapLibre's own `map=` parameter, e.g.
// `#map=15/19.05/72.84&sel=place:311442962&area=makabo&dist=19.11970,72.84640~19.06600,72.86800`.

export type LatLon = [number, number];

export type Selection =
  | { kind: 'place' | 'street' | 'bridge' | 'route' | 'building'; id: number }
  | { kind: 'pin'; lat: number; lon: number };

const SELECTION_KINDS = new Set(['place', 'street', 'bridge', 'route', 'building']);

/** The parameters of a hash string (with or without the leading `#`). */
export function hashParams(hash: string = location.hash): URLSearchParams {
  return new URLSearchParams(hash.startsWith('#') ? hash.slice(1) : hash);
}

/** Serialises parameters, keeping `/ : , ~` readable (they are safe in a fragment). */
export function encodeHash(params: URLSearchParams): string {
  return params.toString().replace(/%2F/gi, '/').replace(/%3A/gi, ':').replace(/%2C/gi, ',').replace(/%7E/gi, '~');
}

/** Sets or removes hash parameters without adding a history entry. */
export function updateHash(changes: Record<string, string | null>): void {
  const params = hashParams();
  for (const [key, value] of Object.entries(changes)) {
    if (value === null || value === '') params.delete(key);
    else params.set(key, value);
  }
  const next = encodeHash(params);
  history.replaceState(history.state, '', `${location.pathname}${location.search}${next ? `#${next}` : ''}`);
}

/** Parses `sel=` values: `place:123`, `street:45`, `pin:19.05,72.84`… Null when malformed. */
export function parseSelection(value: string | null): Selection | null {
  if (!value) return null;
  const colon = value.indexOf(':');
  if (colon < 0) return null;
  const kind = value.slice(0, colon);
  const rest = value.slice(colon + 1);
  if (kind === 'pin') {
    const point = parseLatLon(rest);
    return point ? { kind: 'pin', lat: point[0], lon: point[1] } : null;
  }
  if (!SELECTION_KINDS.has(kind) || !/^\d+$/.test(rest)) return null;
  return { kind: kind as 'place', id: Number(rest) };
}

/** Parses `lat,lon`, rejecting values outside the valid ranges. */
export function parseLatLon(value: string | null | undefined): LatLon | null {
  if (!value) return null;
  const parts = value.split(',');
  if (parts.length !== 2 || parts.some((p) => p.trim() === '')) return null;
  const [lat, lon] = parts.map(Number);
  if (!Number.isFinite(lat) || !Number.isFinite(lon) || Math.abs(lat) > 90 || Math.abs(lon) > 180) return null;
  return [lat, lon];
}

/** Parses a route `dist=lat,lon~lat,lon` parameter. */
export function parseRoute(value: string | null): { from: LatLon; to: LatLon } | null {
  if (!value) return null;
  const ends = value.split('~');
  if (ends.length !== 2) return null;
  const from = parseLatLon(ends[0]);
  const to = parseLatLon(ends[1]);
  return from && to ? { from, to } : null;
}

/** Formats a route parameter with five decimals (about a metre). */
export function formatRoute(from: LatLon, to: LatLon): string {
  const f = (p: LatLon) => `${p[0].toFixed(5)},${p[1].toFixed(5)}`;
  return `${f(from)}~${f(to)}`;
}
