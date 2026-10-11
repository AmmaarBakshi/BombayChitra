const nf = new Intl.NumberFormat('en-IN');
const nf1 = new Intl.NumberFormat('en-IN', { maximumFractionDigits: 1 });

export const formatNumber = (n: number) => nf.format(n);

export function formatDistance(m: number): string {
  if (m < 1000) return `${Math.round(m)} m`;
  return `${nf1.format(m / 1000)} km`;
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
