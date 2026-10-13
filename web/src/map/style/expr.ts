import type { ExpressionSpecification } from '@maplibre/maplibre-gl-style-spec';

/** ["match", ["get", key], v1, out1, …, fallback] from a record. */
export function matchKey<T extends string | number>(key: string, cases: Record<string, T>, fallback: T): ExpressionSpecification {
  const flat: (string | number)[] = [];
  for (const [k, v] of Object.entries(cases)) flat.push(k, v);
  return ['match', ['get', key], ...flat, fallback] as unknown as ExpressionSpecification;
}

/** Exponential zoom interpolation from [zoom, value] stops; values may be data expressions. */
export function zoomExp(base: number, ...stops: [number, number | ExpressionSpecification][]): ExpressionSpecification {
  return ['interpolate', ['exponential', base], ['zoom'], ...stops.flat()] as unknown as ExpressionSpecification;
}

export function zoomLinear(...stops: [number, number | string | ExpressionSpecification][]): ExpressionSpecification {
  return ['interpolate', ['linear'], ['zoom'], ...stops.flat()] as unknown as ExpressionSpecification;
}

/** Road classes in draw order (lowest first). */
export const roadOrder = ['path', 'track', 'service', 'pedestrian', 'minor', 'tertiary', 'secondary', 'primary', 'trunk', 'motorway'] as const;

export const roadSortKey = matchKey('kind', Object.fromEntries(roadOrder.map((k, i) => [k, i])), 0);

export const isBridge: ExpressionSpecification = ['==', ['get', 'brunnel'], 'bridge'];
export const isTunnel: ExpressionSpecification = ['==', ['get', 'brunnel'], 'tunnel'];
export const notBrunnel: ExpressionSpecification = ['!', ['has', 'brunnel']];

/** Languages the map can be labelled in: the default names (mostly English), Marathi or Hindi. */
export type Language = 'en' | 'mr' | 'hi';
export const LANGUAGES: Language[] = ['en', 'mr', 'hi'];

/** Label text: the name in the chosen language where the tiles have it (`name:mr`, `name:hi`), else the name. */
export function labelField(lang: Language = 'en'): ExpressionSpecification {
  return lang === 'en' ? ['coalesce', ['get', 'name'], ''] : ['coalesce', ['get', `name:${lang}`], ['get', 'name'], ''];
}

export const fonts = {
  regular: ['Noto Sans Regular'],
  medium: ['Noto Sans Medium'],
  italic: ['Noto Sans Italic'],
};
