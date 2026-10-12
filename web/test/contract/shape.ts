// Compares JSON values with the shape documented in api/openapi.yaml: a response must have every field of the
// documented example with the same JSON type, recursively (the first item of an array stands for all of them), and
// must satisfy the response schema where one is given.

export type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

export interface Schema {
  type?: string | string[];
  properties?: Record<string, Schema>;
  required?: string[];
  items?: Schema;
  enum?: Json[];
  $ref?: string;
}

export function kind(v: Json | undefined): string {
  if (v === undefined) return 'missing';
  if (v === null) return 'null';
  if (Array.isArray(v)) return 'array';
  return typeof v;
}

/** Fields whose value is a dictionary with data-dependent keys (OSM tags, counts per category…): only their type is checked. */
export const FREE_FORM = new Set(['tags', 'counts', 'totals', 'places_by_category', 'stats', 'links', 'detail']);

/** Differences between a documented example and an actual response, as "path: problem" strings. */
export function compareShape(example: Json, actual: Json | undefined, path = '$'): string[] {
  const ek = kind(example);
  const ak = kind(actual);
  if (ek === 'null') return [];
  if (ek !== ak) return [`${path}: documented as ${ek}, got ${ak}`];
  if (ek === 'array') {
    const [e] = example as Json[];
    const [a] = actual as Json[];
    // Placeholders like "…" in examples mark elided items, not a string type.
    if (e === undefined || a === undefined || e === '…') return [];
    return compareShape(e, a, `${path}[0]`);
  }
  if (ek === 'object') {
    if (FREE_FORM.has(path.slice(path.lastIndexOf('.') + 1))) return [];
    const out: string[] = [];
    for (const [key, value] of Object.entries(example as Record<string, Json>)) {
      out.push(...compareShape(value, (actual as Record<string, Json>)[key], `${path}.${key}`));
    }
    return out;
  }
  return [];
}

/** Validates a value against the subset of JSON Schema the spec uses. */
export function validate(schema: Schema, value: Json | undefined, resolve: (ref: string) => Schema, path = '$'): string[] {
  if (schema.$ref) return validate(resolve(schema.$ref), value, resolve, path);
  const out: string[] = [];
  const types = schema.type === undefined ? [] : Array.isArray(schema.type) ? schema.type : [schema.type];
  const k = kind(value);
  const matches = (t: string) => t === k || (t === 'integer' && k === 'number' && Number.isInteger(value)) || (t === 'number' && k === 'number');
  if (types.length && !types.some(matches)) return [`${path}: expected ${types.join(' | ')}, got ${k}`];
  if (schema.enum && !schema.enum.includes(value as Json)) out.push(`${path}: ${JSON.stringify(value)} is not one of ${JSON.stringify(schema.enum)}`);
  if (k === 'object') {
    const obj = value as Record<string, Json>;
    for (const name of schema.required ?? []) if (!(name in obj)) out.push(`${path}: missing required ${name}`);
    for (const [name, sub] of Object.entries(schema.properties ?? {})) {
      if (name in obj) out.push(...validate(sub, obj[name], resolve, `${path}.${name}`));
    }
  }
  if (k === 'array' && schema.items) {
    (value as Json[]).forEach((item, i) => out.push(...validate(schema.items!, item, resolve, `${path}[${i}]`)));
  }
  return out;
}
