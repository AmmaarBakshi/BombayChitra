// Inlines the Maki icon paths used by config/categories.json as a virtual module, so the app ships only the icons it
// draws (as Path2D data) instead of loading SVG files.
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import type { Plugin } from 'vite';

export interface IconPath {
  d: string;
  evenodd?: boolean;
}

/** Icons that Maki lacks, drawn on the same 15×15 grid. */
const CUSTOM: Record<string, IconPath> = {
  atm: {
    d: 'M1.5 3.5h12a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-12a1 1 0 0 1-1-1v-6a1 1 0 0 1 1-1zM7.5 5.5a2 2 0 1 0 0 4a2 2 0 1 0 0-4zM2.5 5v1.2h1.2V5zM11.3 8.8V10h1.2V8.8z',
    evenodd: true,
  },
};

const VIRTUAL_ID = 'virtual:bc-icons';
const RESOLVED_ID = '\0' + VIRTUAL_ID;

function decodeEntities(s: string): string {
  return s
    .replace(/&#x([0-9a-f]+);/gi, (_, hex: string) => String.fromCodePoint(parseInt(hex, 16)))
    .replace(/&#(\d+);/g, (_, dec: string) => String.fromCodePoint(parseInt(dec, 10)))
    .replace(/\s+/g, ' ')
    .trim();
}

export function loadIcons(categoriesPath: string, extra: string[] = []): Record<string, IconPath> {
  const require = createRequire(import.meta.url);
  const makiDir = path.join(path.dirname(require.resolve('@mapbox/maki/package.json')), 'icons');
  const config = JSON.parse(readFileSync(categoriesPath, 'utf8')) as { groups: { categories: { icon: string }[] }[] };
  const names = new Set<string>(extra);
  for (const g of config.groups) for (const c of g.categories) names.add(c.icon);
  const icons: Record<string, IconPath> = {};
  for (const name of [...names].sort()) {
    if (CUSTOM[name]) {
      icons[name] = CUSTOM[name];
      continue;
    }
    const svg = readFileSync(path.join(makiDir, `${name}.svg`), 'utf8');
    const ds = [...svg.matchAll(/\sd="([^"]+)"/g)].map((m) => decodeEntities(m[1]));
    if (ds.length === 0) throw new Error(`Maki icon ${name} has no path`);
    icons[name] = { d: ds.join(' ') };
  }
  return icons;
}

export function iconsPlugin(categoriesPath: string, extra: string[] = []): Plugin {
  return {
    name: 'bombaychitra-icons',
    resolveId(id) {
      return id === VIRTUAL_ID ? RESOLVED_ID : null;
    },
    load(id) {
      if (id !== RESOLVED_ID) return null;
      this.addWatchFile(categoriesPath);
      return `export default ${JSON.stringify(loadIcons(categoriesPath, extra))};`;
    },
  };
}
