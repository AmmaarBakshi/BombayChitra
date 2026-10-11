// A small, dependency-free client for the BombayChitra REST API. It works in browsers and Node 18+ (global fetch) and
// can be copied into any JS/TS project; for other languages generate a client from /openapi.json.
import type {
  Area,
  BBox,
  Building,
  CategoryGroup,
  ListResponse,
  Meta,
  NamedBridge,
  Place,
  PlaceDetail,
  Reverse,
  Road,
  RouteResult,
  SearchKind,
  SearchResult,
  Stats,
  Street,
  StreetDetail,
  TransitLine,
  TransitRoute,
  Bridge,
} from './types.ts';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type Query = Record<string, string | number | boolean | string[] | BBox | undefined | null>;

export interface PlaceFilter {
  bbox?: BBox;
  category?: string[];
  group?: string[];
  area?: string;
  q?: string;
  named?: boolean;
  limit?: number;
  offset?: number;
}

export class BombayChitraClient {
  private readonly base: string;

  /** @param base origin of a BombayChitra server, e.g. "https://maps.example.com" ("" = same origin). */
  constructor(base = '', private readonly init: RequestInit = {}) {
    this.base = base.replace(/\/+$/, '');
  }

  async get<T>(path: string, query: Query = {}, signal?: AbortSignal): Promise<T> {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === '' || (Array.isArray(value) && value.length === 0)) continue;
      params.set(key, Array.isArray(value) ? value.join(',') : String(value));
    }
    const qs = params.toString();
    const res = await fetch(`${this.base}/api/v1${path}${qs ? `?${qs}` : ''}`, { ...this.init, signal });
    if (!res.ok) {
      let code = 'http_error';
      let message = res.statusText;
      try {
        const body = (await res.json()) as { error?: { code: string; message: string } };
        if (body.error) ({ code, message } = body.error);
      } catch {
        /* non-JSON error */
      }
      throw new ApiError(res.status, code, message);
    }
    return (await res.json()) as T;
  }

  meta(signal?: AbortSignal) {
    return this.get<Meta>('/meta', {}, signal);
  }

  categories(signal?: AbortSignal) {
    return this.get<{ groups: CategoryGroup[] }>('/categories', {}, signal);
  }

  areas(signal?: AbortSignal) {
    return this.get<{ items: Area[] }>('/areas', {}, signal);
  }

  area(id: string, signal?: AbortSignal) {
    return this.get<Area>(`/areas/${encodeURIComponent(id)}`, {}, signal);
  }

  stats(area?: string, signal?: AbortSignal) {
    return this.get<Stats>('/stats', { area }, signal);
  }

  search(q: string, opts: { limit?: number; near?: [number, number]; kind?: SearchKind[] } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<SearchResult> & { query: string }>(
      '/search',
      { q, limit: opts.limit, kind: opts.kind, lon: opts.near?.[0], lat: opts.near?.[1] },
      signal,
    );
  }

  reverse(lon: number, lat: number, signal?: AbortSignal) {
    return this.get<Reverse>('/reverse', { lon, lat }, signal);
  }

  places(filter: PlaceFilter = {}, signal?: AbortSignal) {
    return this.get<ListResponse<Place>>('/places', { ...filter }, signal);
  }

  nearby(lon: number, lat: number, opts: PlaceFilter & { radius?: number } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<Place>>('/nearby', { ...opts, lon, lat }, signal);
  }

  place(id: number | string, signal?: AbortSignal) {
    return this.get<PlaceDetail>(`/places/${id}`, {}, signal);
  }

  streets(opts: { q?: string; bbox?: BBox; area?: string; kind?: string[]; limit?: number } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<Street>>('/streets', opts, signal);
  }

  street(id: number, signal?: AbortSignal) {
    return this.get<StreetDetail>(`/streets/${id}`, {}, signal);
  }

  road(id: number, signal?: AbortSignal) {
    return this.get<Road>(`/roads/${id}`, {}, signal);
  }

  bridges(opts: { bbox?: BBox; kind?: string[]; min_length?: number; area?: string; q?: string; limit?: number } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<Bridge>>('/bridges', opts, signal);
  }

  bridge(id: number, signal?: AbortSignal) {
    return this.get<Bridge>(`/bridges/${id}`, {}, signal);
  }

  namedBridges(opts: { bbox?: BBox; kind?: string[]; min_length?: number; area?: string; q?: string; limit?: number } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<NamedBridge>>('/bridges/named', opts, signal);
  }

  namedBridge(id: number, signal?: AbortSignal) {
    return this.get<NamedBridge>(`/bridges/named/${id}`, {}, signal);
  }

  buildings(opts: { bbox?: BBox; area?: string; min_levels?: number; min_height?: number; named?: boolean; sort?: string; limit?: number } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<Building>>('/buildings', opts, signal);
  }

  building(id: number, signal?: AbortSignal) {
    return this.get<Building>(`/buildings/${id}`, {}, signal);
  }

  transitLines(opts: { mode?: string[]; network?: string } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<TransitLine>>('/transit/lines', opts, signal);
  }

  transitRoute(id: number, signal?: AbortSignal) {
    return this.get<TransitRoute>(`/transit/routes/${id}`, {}, signal);
  }

  stations(opts: { mode?: 'metro' | 'monorail' | 'railway'; area?: string; bbox?: BBox } = {}, signal?: AbortSignal) {
    return this.get<ListResponse<PlaceDetail>>('/transit/stations', opts, signal);
  }

  /** Driving route between two lon/lat points ("distance by road"). */
  route(from: [number, number], to: [number, number], optimize: 'time' | 'distance' = 'time', signal?: AbortSignal) {
    const pair = (p: [number, number]) => `${p[1].toFixed(6)},${p[0].toFixed(6)}`;
    return this.get<RouteResult>('/route', { from: pair(from), to: pair(to), optimize }, signal);
  }
}
