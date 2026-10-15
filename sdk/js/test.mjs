// Tests of the generated JavaScript client with a stub fetch (no server needed): node --test sdk/js
import assert from 'node:assert/strict';
import test from 'node:test';
import { BombayChitra, BombayChitraError } from './index.js';

function stub(status = 200, body = {}, contentType = 'application/json') {
  const calls = [];
  const fetch = async (url, init) => {
    calls.push({ url: new URL(url), init });
    return new Response(typeof body === 'string' ? body : JSON.stringify(body), { status, headers: { 'Content-Type': contentType } });
  };
  return { calls, api: new BombayChitra('http://maps.example.com/', { apiKey: 'secret', fetch }) };
}

test('builds query strings, joining lists and skipping missing options', async () => {
  const { calls, api } = stub();
  await api.listPlaces({ category: ['hospital', 'clinic'], named: true, area: undefined });
  const { url, init } = calls[0];
  assert.equal(url.pathname, '/api/v1/places');
  assert.equal(url.searchParams.get('category'), 'hospital,clinic');
  assert.equal(url.searchParams.get('named'), 'true');
  assert.equal(url.searchParams.has('area'), false);
  assert.equal(init.headers['X-API-Key'], 'secret');
});

test('puts required parameters and path ids in place', async () => {
  const { calls, api } = stub();
  await api.route('19.1197,72.8464', '18.94,72.8355', { mode: 'foot' });
  await api.getArea('ward-f/n');
  assert.equal(calls[0].url.searchParams.get('from'), '19.1197,72.8464');
  assert.equal(calls[1].url.pathname, '/api/v1/areas/ward-f%2Fn');
});

test('posts JSON bodies', async () => {
  const { calls, api } = stub();
  await api.matrix({ sources: [{ lat: 19.1, lon: 72.85 }] });
  assert.equal(calls[0].init.method, 'POST');
  assert.equal(calls[0].init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(calls[0].init.body), { sources: [{ lat: 19.1, lon: 72.85 }] });
});

test('turns error envelopes into BombayChitraError', async () => {
  const { api } = stub(422, { error: { status: 422, code: 'no_road', message: 'no drivable road' } });
  await assert.rejects(api.route('19.1,72.8', '18.5,72.0'), (err) => err instanceof BombayChitraError && err.status === 422 && err.code === 'no_road');
  const html = stub(502, '<html>Bad gateway</html>', 'text/html');
  await assert.rejects(html.api.getMeta(), (err) => err.status === 502 && err.code === 'http_error');
});
