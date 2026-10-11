import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { loadIcons } from '../build/icons.ts';
import { categories, categoryById, groupById, groups, iconNames } from '../src/taxonomy.ts';

const config = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../config/categories.json');

describe('taxonomy', () => {
  it('flattens groups into categories that carry their group colour', () => {
    expect(categories.length).toBe(groups.reduce((n, g) => n + g.categories.length, 0));
    for (const c of categories) {
      expect(groupById.get(c.group)?.colour).toBe(c.colour);
    }
  });

  it('has unique category ids', () => {
    expect(new Set(categories.map((c) => c.id)).size).toBe(categories.length);
    expect(categoryById.get('hospital')?.group).toBe('health');
    expect(categoryById.get('metro_station')?.label).toBe('Metro stations');
  });

  it('uses valid colours, ranks and zoom levels', () => {
    for (const g of groups) expect(g.colour).toMatch(/^#[0-9a-f]{6}$/i);
    for (const c of categories) {
      expect(c.rank).toBeGreaterThanOrEqual(1);
      expect(c.rank).toBeLessThanOrEqual(5);
      expect(c.minzoom).toBeGreaterThanOrEqual(8);
      expect(c.minzoom).toBeLessThanOrEqual(16);
    }
  });

  it('only references icons that exist', () => {
    const icons = loadIcons(config);
    for (const name of iconNames) expect(icons[name]?.d, name).toBeTruthy();
  });
});
