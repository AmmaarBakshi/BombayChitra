import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, BombayChitraClient } from '../src/api/client.ts';

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn(async (_url: string, _init?: RequestInit) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }));
  vi.stubGlobal('fetch', fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('BombayChitraClient', () => {
  it('builds query strings, joining lists and skipping empty values', async () => {
    const fetch = mockFetch(200, { count: 0, items: [] });
    const api = new BombayChitraClient('https://maps.example.com/');
    await api.places({ category: ['hospital', 'clinic'], area: 'makabo', q: '', bbox: [72.8, 19.0, 72.9, 19.1], limit: 5 });
    expect(fetch).toHaveBeenCalledOnce();
    const url = new URL(fetch.mock.calls[0][0]);
    expect(url.origin + url.pathname).toBe('https://maps.example.com/api/v1/places');
    expect(url.searchParams.get('category')).toBe('hospital,clinic');
    expect(url.searchParams.get('bbox')).toBe('72.8,19,72.9,19.1');
    expect(url.searchParams.get('limit')).toBe('5');
    expect(url.searchParams.has('q')).toBe(false);
  });

  it('sends route ends as lat,lon', async () => {
    const fetch = mockFetch(200, {});
    await new BombayChitraClient('').route([72.8464, 19.1197], [72.868, 19.066], { optimize: 'distance', mode: 'foot', avoid: ['toll'] });
    const url = new URL(fetch.mock.calls[0][0], 'http://localhost');
    expect(url.searchParams.get('from')).toBe('19.119700,72.846400');
    expect(url.searchParams.get('to')).toBe('19.066000,72.868000');
    expect(url.searchParams.get('optimize')).toBe('distance');
    expect(url.searchParams.get('mode')).toBe('foot');
    expect(url.searchParams.get('avoid')).toBe('toll');
    expect(url.searchParams.has('steps')).toBe(false);
  });

  it('asks for journeys with modes and a walking limit', async () => {
    const fetch = mockFetch(200, {});
    await new BombayChitraClient('').journey([72.843, 19.019], [72.9087, 19.0864], { modes: ['train', 'metro'], maxWalk: 800 });
    const url = new URL(fetch.mock.calls[0][0], 'http://localhost');
    expect(url.pathname).toBe('/api/v1/journey');
    expect(url.searchParams.get('from')).toBe('19.019000,72.843000');
    expect(url.searchParams.get('modes')).toBe('train,metro');
    expect(url.searchParams.get('max_walk')).toBe('800');
  });

  it('posts matrices and batch searches as JSON', async () => {
    const fetch = mockFetch(200, {});
    const api = new BombayChitraClient('', { headers: { 'X-API-Key': 'k' } });
    await api.matrix([[72.8464, 19.1197], [72.8402, 19.0544]], undefined, { mode: 'foot' });
    let [url, init] = fetch.mock.calls[0];
    expect(url).toBe('/api/v1/matrix');
    expect(init?.method).toBe('POST');
    expect(new Headers(init?.headers).get('Content-Type')).toBe('application/json');
    expect(new Headers(init?.headers).get('X-API-Key')).toBe('k');
    expect(JSON.parse(init?.body as string)).toEqual({ sources: [{ lat: 19.1197, lon: 72.8464 }, { lat: 19.0544, lon: 72.8402 }], mode: 'foot' });

    await api.searchBatch(['CST', 'Bandra'], { near: [72.87, 19.07] });
    [url, init] = fetch.mock.calls[1];
    expect(url).toBe('/api/v1/search/batch');
    expect(JSON.parse(init?.body as string)).toEqual({ queries: ['CST', 'Bandra'], near: { lon: 72.87, lat: 19.07 } });
  });

  it('turns error envelopes into ApiError', async () => {
    mockFetch(422, { error: { status: 422, code: 'no_route', message: 'the points are not connected by drivable roads' } });
    const err = await new BombayChitraClient('').route([72.8, 19.0], [72.9, 19.1]).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 422, code: 'no_route', message: 'the points are not connected by drivable roads' });
  });

  it('copes with non-JSON errors', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Bad gateway</html>', { status: 502, statusText: 'Bad Gateway' })));
    const err = await new BombayChitraClient('').meta().catch((e: unknown) => e);
    expect(err).toMatchObject({ status: 502, code: 'http_error' });
  });

  it('passes request options such as headers', async () => {
    const fetch = mockFetch(200, {});
    await new BombayChitraClient('', { headers: { 'X-API-Key': 'k' } }).meta();
    expect(fetch.mock.calls[0][1]?.headers).toEqual({ 'X-API-Key': 'k' });
  });
});
