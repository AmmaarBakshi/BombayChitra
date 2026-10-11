// Writes .br and .gz siblings for compressible build output so the Go server can serve them without compressing on
// every request.
import { readdir, readFile, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { brotliCompressSync, constants, gzipSync } from 'node:zlib';
import type { Plugin } from 'vite';

const COMPRESSIBLE = new Set(['.js', '.mjs', '.css', '.html', '.json', '.svg', '.txt', '.yaml', '.xml', '.webmanifest', '.pbf', '.map']);
const MIN_SIZE = 1024;

async function* walk(dir: string): AsyncGenerator<string> {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(full);
    else yield full;
  }
}

export function precompressPlugin(outDir: string): Plugin {
  return {
    name: 'bombaychitra-precompress',
    apply: 'build',
    async closeBundle() {
      let files = 0;
      let before = 0;
      let after = 0;
      for await (const file of walk(outDir)) {
        if (!COMPRESSIBLE.has(path.extname(file))) continue;
        const { size } = await stat(file);
        if (size < MIN_SIZE) continue;
        const data = await readFile(file);
        const br = brotliCompressSync(data, { params: { [constants.BROTLI_PARAM_QUALITY]: 11, [constants.BROTLI_PARAM_SIZE_HINT]: size } });
        const gz = gzipSync(data, { level: 9 });
        if (br.length < size * 0.95) await writeFile(`${file}.br`, br);
        if (gz.length < size * 0.95) await writeFile(`${file}.gz`, gz);
        files++;
        before += size;
        after += br.length;
      }
      // Keep the placeholder so `go build` works from a clean checkout.
      await writeFile(path.join(outDir, '.gitkeep'), '');
      console.log(`precompressed ${files} files: ${(before / 1e6).toFixed(2)} MB → ${(after / 1e6).toFixed(2)} MB (brotli)`);
    },
  };
}
