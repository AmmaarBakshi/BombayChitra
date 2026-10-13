// Emits the portable light/dark MapLibre styles (styles/light.json, styles/dark.json, and Marathi and Hindi labelled
// variants styles/light-mr.json … styles/dark-hi.json) and validates every style variant against the style
// specification at build time. The Go server substitutes {{BASE}} with its public origin.
import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec';
import type { Plugin } from 'vite';
import { buildStyle, type Language, LANGUAGES } from '../src/map/style/index.ts';
import type { ThemeName } from '../src/map/style/palette.ts';

const THEMES: ThemeName[] = ['light', 'dark'];

/** File name of a portable style: light.json, dark-mr.json… */
const fileName = (theme: ThemeName, lang: Language) => `${theme}${lang === 'en' ? '' : `-${lang}`}.json`;

function validated(theme: ThemeName, app: boolean, base: string, lang: Language = 'en') {
  const style = buildStyle({ theme, base, app, lang });
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
        for (const lang of LANGUAGES) {
          this.emitFile({ type: 'asset', fileName: `styles/${fileName(theme, lang)}`, source: JSON.stringify(validated(theme, false, '{{BASE}}', lang)) });
        }
      }
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const m = req.url?.match(/^\/styles\/(light|dark)(?:-(mr|hi))?\.json/);
        if (!m) return next();
        const base = `http://${req.headers.host}`;
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(validated(m[1] as ThemeName, false, base, (m[2] ?? 'en') as Language)));
      });
    },
  };
}
