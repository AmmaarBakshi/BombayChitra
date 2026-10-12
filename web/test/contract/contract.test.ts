// API contract test: calls a running server with every example request in api/openapi.yaml and checks each response
// against the documented example and schema, so the reference and the server cannot drift apart.
//
//   BC_API_URL=http://localhost:8080 npm run test:contract
//
// Skipped when BC_API_URL is not set (it needs a server with data).
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeAll, describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import { compareShape, type Json, type Schema, validate } from './shape.ts';

interface Param {
  name: string;
  in: string;
  required?: boolean;
  example?: Json;
  $ref?: string;
}

interface Operation {
  operationId?: string;
  parameters?: Param[];
  responses?: Record<string, { content?: Record<string, { example?: Json; schema?: Schema }> } | { $ref: string }>;
}

interface Spec {
  paths: Record<string, Record<string, Operation>>;
  components: { parameters: Record<string, Param>; schemas: Record<string, Schema> };
}

const base = process.env.BC_API_URL?.replace(/\/+$/, '');
const here = path.dirname(fileURLToPath(import.meta.url));
const spec = parse(readFileSync(path.join(here, '../../../api/openapi.yaml'), 'utf8')) as Spec;
const resolveParam = (p: Param): Param => (p.$ref ? { ...spec.components.parameters[p.$ref.split('/').pop()!], ...p } : p);
const resolveSchema = (ref: string): Schema => spec.components.schemas[ref.split('/').pop()!];

interface Case {
  name: string;
  url: string;
  example?: Json;
  schema?: Schema;
}

/** One request per documented GET operation whose parameters all have examples (or are optional). */
function cases(): Case[] {
  const out: Case[] = [];
  for (const [route, methods] of Object.entries(spec.paths)) {
    const op = methods.get;
    if (!op || route.startsWith('/tiles/') || route.startsWith('/styles/')) continue;
    const params = (op.parameters ?? []).map(resolveParam);
    if (params.some((p) => p.required && p.example === undefined)) continue;
    let url = route;
    const query = new URLSearchParams();
    for (const p of params) {
      if (p.example === undefined) continue;
      if (p.in === 'path') url = url.replace(`{${p.name}}`, encodeURIComponent(String(p.example)));
      else if (p.in === 'query') query.set(p.name, String(p.example));
    }
    if (url.includes('{')) continue;
    const ok = op.responses?.['200'];
    const content = ok && !('$ref' in ok) ? Object.values(ok.content ?? {})[0] : undefined;
    out.push({ name: `${op.operationId ?? route} GET ${url}`, url: `${url}${query.size ? `?${query}` : ''}`, example: content?.example, schema: content?.schema });
  }
  return out;
}

describe.skipIf(!base)('API contract', () => {
  // Load the routing graph first, so /api/v1/health reports it as documented whatever order requests come in.
  beforeAll(async () => {
    await fetch(`${base}/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680&steps=false`);
  });

  it.each(cases())('$name', async ({ url, example, schema }) => {
    const res = await fetch(`${base}${url}`);
    expect(res.status, `GET ${url}`).toBe(200);
    expect(res.headers.get('content-type') ?? '').toMatch(/json/);
    const body = (await res.json()) as Json;
    if (example !== undefined) expect(compareShape(example, body), `GET ${url} differs from the documented example`).toEqual([]);
    if (schema) expect(validate(schema, body, resolveSchema), `GET ${url} breaks the response schema`).toEqual([]);
  });

  it('documents every endpoint the server lists', async () => {
    const index = (await (await fetch(`${base}/api/v1`)).json()) as { endpoints: string[] };
    const documented = new Set(Object.keys(spec.paths));
    const missing = index.endpoints.filter((e) => e.startsWith('/api/') && !documented.has(e));
    expect(missing, 'endpoints missing from api/openapi.yaml').toEqual([]);
  });
});
