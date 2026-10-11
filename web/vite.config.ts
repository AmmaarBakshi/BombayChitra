import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import { docsPlugin } from './build/docs.ts';
import { iconsPlugin } from './build/icons.ts';
import { precompressPlugin } from './build/precompress.ts';
import { stylesPlugin } from './build/styles.ts';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..');
const outDir = path.join(repo, 'server', 'internal', 'webui', 'dist');
// The Go server the dev server proxies API and tile requests to.
const backend = process.env.BC_BACKEND ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [
    iconsPlugin(path.join(repo, 'config', 'categories.json'), ['marker']),
    stylesPlugin(),
    docsPlugin({ docsDir: path.join(repo, 'docs'), openapi: path.join(repo, 'api', 'openapi.yaml'), repoUrl: 'https://github.com/AmmaarBakshi/BombayChitra' }),
    precompressPlugin(outDir),
  ],
  build: {
    outDir,
    emptyOutDir: true,
    target: 'es2022',
    chunkSizeWarningLimit: 1500,
    assetsInlineLimit: 0,
    rolldownOptions: {
      // MapLibre changes far less often than the app: keep it in its own long-cacheable chunk.
      output: { codeSplitting: { groups: [{ name: 'maplibre', test: /node_modules[\/]maplibre-gl/ }] } },
    },
  },
  server: {
    port: 5173,
    fs: { allow: [repo] },
    proxy: Object.fromEntries(['/api', '/tiles', '/data', '/healthz'].map((p) => [p, { target: backend, changeOrigin: true }])),
  },
});
