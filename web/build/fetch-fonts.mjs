#!/usr/bin/env node
// Downloads the glyph ranges the map styles use (Noto Sans, SIL OFL 1.1) into web/public/fonts.
// Run once after cloning or when adding fonts/ranges: `node build/fetch-fonts.mjs`.
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SOURCE = 'https://protomaps.github.io/basemaps-assets/fonts';
const FONTS = ['Noto Sans Regular', 'Noto Sans Medium', 'Noto Sans Italic'];
// Latin, Latin Extended, Devanagari, punctuation, arrows/letterlike symbols.
const RANGES = ['0-255', '256-511', '2304-2559', '8192-8447', '8448-8703'];

const outDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'public', 'fonts');

async function fetchBytes(url) {
  for (let attempt = 1; ; attempt++) {
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return new Uint8Array(await res.arrayBuffer());
    } catch (err) {
      if (attempt >= 3) throw new Error(`${url}: ${err.message}`);
      await new Promise((r) => setTimeout(r, 500 * attempt));
    }
  }
}

let total = 0;
for (const font of FONTS) {
  await mkdir(path.join(outDir, font), { recursive: true });
  await Promise.all(
    RANGES.map(async (range) => {
      const data = await fetchBytes(`${SOURCE}/${encodeURIComponent(font)}/${range}.pbf`);
      await writeFile(path.join(outDir, font, `${range}.pbf`), data);
      total += data.length;
    }),
  );
}
await writeFile(path.join(outDir, 'OFL.txt'), await fetchBytes(`${SOURCE}/OFL.txt`));
console.log(`fonts: ${FONTS.length} × ${RANGES.length} ranges, ${(total / 1e6).toFixed(2)} MB → ${outDir}`);
