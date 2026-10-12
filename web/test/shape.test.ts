import { describe, expect, it } from 'vitest';
import { compareShape, validate, type Schema } from './contract/shape.ts';

describe('compareShape', () => {
  it('accepts responses with the documented fields and types', () => {
    const example = { count: 2, items: [{ id: 1, name: 'A', tags: { a: 'b' } }], note: null };
    const actual = { count: 7, items: [{ id: 9, name: 'Z', tags: {}, extra: true }], more: 1 };
    expect(compareShape(example, actual)).toEqual([]);
  });

  it('reports missing fields and changed types with their path', () => {
    const example = { count: 2, items: [{ id: 1, name: 'A' }], bbox: [1, 2, 3, 4] };
    const actual = { count: '2', items: [{ id: 1 }], bbox: 'none' };
    expect(compareShape(example, actual)).toEqual([
      '$.count: documented as number, got string',
      '$.items[0].name: documented as string, got missing',
      '$.bbox: documented as array, got string',
    ]);
  });

  it('treats empty arrays and elided items as unconstrained', () => {
    expect(compareShape({ a: [] }, { a: [1, 2] })).toEqual([]);
    expect(compareShape({ a: [{ x: 1 }] }, { a: [] })).toEqual([]);
    expect(compareShape({ a: ['…'] }, { a: [[1, 2]] })).toEqual([]);
  });
});

describe('validate', () => {
  const schemas: Record<string, Schema> = {
    Place: { type: 'object', required: ['id', 'lon'], properties: { id: { type: 'integer' }, lon: { type: 'number' }, osm_type: { type: 'string', enum: ['node', 'way'] } } },
  };
  const resolve = (ref: string) => schemas[ref.split('/').pop()!];

  it('checks types, required properties, enums and items', () => {
    const list: Schema = { type: 'array', items: { $ref: '#/components/schemas/Place' } };
    expect(validate(list, [{ id: 1, lon: 72.8, osm_type: 'node' }], resolve)).toEqual([]);
    expect(validate(list, [{ id: 1.5, osm_type: 'tram' }], resolve)).toEqual([
      '$[0]: missing required lon',
      '$[0].id: expected integer, got number',
      '$[0].osm_type: "tram" is not one of ["node","way"]',
    ]);
  });
});
