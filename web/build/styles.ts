// Emits the portable light/dark MapLibre styles (styles/light.json, styles/dark.json) and validates every style
// variant against the style specification at build time. The Go server substitutes {{BASE}} with its public origin.
import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec';
import type { Plugin } from 'vite';
import { buildStyle } from '../src/map/style/index.ts';
import type { ThemeName } from '../src/map/style/palette.ts';

const THEMES: ThemeName[] = ['light', 'dark'];

function validated(theme: ThemeName, app: boolean, base: string) {
  const style = buildStyle({ theme, base, app });
  const errors = validateStyleMin(style as Parameters<typeof validateStyleMin>[0]);
  if (errors.length) {
    throw new Error(`Invalid ${theme}${app ? ' (app)' : ''} style:\n` + errors.map((e) => `  - ${e.message}`).join('\n'));
  }
  return style;
}

export function stylesPlugin(): Plugin {
  return {
    name: 'bombaychitra-styles',
    buildStart() {
      for (const theme of THEMES) validated(theme, true, 'https://example.com');
    },
    generateBundle() {
      for (const theme of THEMES) {
        this.emitFile({ type: 'asset', fileName: `styles/${theme}.json`, source: JSON.stringify(validated(theme, false, '{{BASE}}')) });
      }
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const m = req.url?.match(/^\/styles\/(light|dark)\.json/);
        if (!m) return next();
        const base = `http://${req.headers.host}`;
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(validated(m[1] as ThemeName, false, base)));
      });
    },
  };
}
