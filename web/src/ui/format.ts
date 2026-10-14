const nf = new Intl.NumberFormat('en-IN');
const nf1 = new Intl.NumberFormat('en-IN', { maximumFractionDigits: 1 });

export const formatNumber = (n: number) => nf.format(n);

export function formatDistance(m: number): string {
  if (m < 1000) return `${Math.round(m)} m`;
  return `${nf1.format(m / 1000)} km`;
}

/** Travel time rounded to the minute: "1 min", "45 min", "1 h", "2 h 5 min". */
export function formatDuration(seconds: number): string {
  const minutes = Math.max(1, Math.round(seconds / 60));
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  return minutes % 60 ? `${hours} h ${minutes % 60} min` : `${hours} h`;
}

/** A storage size: "850 KB", "12.4 MB", "10 GB". */
export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${nf.format(Math.max(1, Math.round(bytes / 1024)))} KB`;
  if (bytes < 1024 ** 3) return `${nf1.format(bytes / (1024 * 1024))} MB`;
  return `${nf1.format(bytes / 1024 ** 3)} GB`;
}

export function formatArea(m2: number): string {
  if (m2 < 10_000) return `${nf.format(Math.round(m2))} m²`;
  if (m2 < 1_000_000) return `${nf1.format(m2 / 10_000)} ha`;
  return `${nf1.format(m2 / 1_000_000)} km²`;
}

export function formatCoord(lon: number, lat: number, digits = 6): string {
  return `${lat.toFixed(digits)}, ${lon.toFixed(digits)}`;
}

export function formatDMS(lon: number, lat: number): string {
  const dms = (v: number, pos: string, neg: string) => {
    const a = Math.abs(v);
    const d = Math.floor(a);
    const m = Math.floor((a - d) * 60);
    const s = ((a - d - m / 60) * 3600).toFixed(1);
    return `${d}°${m}′${s}″${v >= 0 ? pos : neg}`;
  };
  return `${dms(lat, 'N', 'S')} ${dms(lon, 'E', 'W')}`;
}

export function formatDate(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

/** Parses an OpenStreetMap date (`2025`, `2025-10`, `2025-10-08`, optionally `~2025`) to its first day, or null. */
export function parseOsmDate(value: string | undefined): Date | null {
  const m = /^~?\s*(\d{4})(?:-(\d{2})(?:-(\d{2}))?)?$/.exec(value?.trim() ?? '');
  if (!m) return null;
  const date = new Date(Date.UTC(Number(m[1]), m[2] ? Number(m[2]) - 1 : 0, m[3] ? Number(m[3]) : 1));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Formats an OpenStreetMap date at the precision it was tagged with: "1853", "Oct 2025", "8 Oct 2025". */
export function formatOsmDate(value: string): string {
  const date = parseOsmDate(value);
  if (!date) return value;
  const parts = value.replace(/^~\s*/, '').split('-').length;
  const opts: Intl.DateTimeFormatOptions = parts === 1 ? { year: 'numeric' } : parts === 2 ? { month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short', year: 'numeric' };
  return date.toLocaleDateString('en-IN', { ...opts, timeZone: 'UTC' });
}

/**
 * Whether something with this opening date opened within `days` before `now` — the "recently opened" badge for new
 * metro stations, the airport and the like. Future dates are not recent (they are planned openings).
 */
export function isRecent(value: string | undefined, now = new Date(), days = 730): boolean {
  const date = parseOsmDate(value);
  if (!date) return false;
  const age = (now.getTime() - date.getTime()) / 86_400_000;
  return age >= 0 && age <= days;
}

export function capitalise(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1).replace(/_/g, ' ') : s;
}

/** Haversine distance in metres. */
export function distance(a: [number, number], b: [number, number]): number {
  const R = 6371008.8;
  const toRad = (d: number) => (d * Math.PI) / 180;
  const dLat = toRad(b[1] - a[1]);
  const dLon = toRad(b[0] - a[0]);
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(toRad(a[1])) * Math.cos(toRad(b[1])) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.min(1, Math.sqrt(h)));
}
