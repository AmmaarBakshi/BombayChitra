// Emits the service worker (build/sw.js → sw.js at the site root) with the files of the app shell to cache at
// install: the page, the bundled scripts and styles, the icon and manifest, the offline page and the map fonts. Its
// version is a hash of that list, so every deployment with new assets installs a fresh shell.
import { createHash } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import type { Plugin } from 'vite';

/** The map font files under public/<sub>, as URL paths from the site root. */
function publicFiles(publicDir: string, sub: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(path.join(publicDir, sub), { withFileTypes: true })) {
    const rel = path.posix.join(sub, entry.name);
    if (entry.isDirectory()) out.push(...publicFiles(publicDir, rel));
    else if (entry.name.endsWith('.pbf')) out.push(encodeURI(`/${rel}`)); // as requested: /fonts/Noto%20Sans%20Regular/…
  }
  return out;
}

export function serviceWorkerPlugin(publicDir: string): Plugin {
  const template = readFileSync(new URL('./sw.js', import.meta.url), 'utf8');
  return {
    name: 'bombaychitra-sw',
    apply: 'build',
    generateBundle(_options, bundle) {
      const assets = Object.keys(bundle).filter((f) => f.startsWith('assets/') && /\.(m?js|css)$/.test(f)).map((f) => `/${f}`);
      const precache = ['/', '/favicon.svg', '/manifest.webmanifest', ...assets, ...publicFiles(publicDir, 'fonts')].sort();
      const version = createHash('sha256').update(template).update(precache.join('\n')).digest('hex').slice(0, 12);
      const source = template.replace("'__BC_VERSION__'", JSON.stringify(version)).replace('__BC_PRECACHE__', JSON.stringify(precache, null, 2));
      this.emitFile({ type: 'asset', fileName: 'sw.js', source });
    },
  };
}
