# web

The BombayChitra map app and documentation site: TypeScript, [MapLibre GL JS](https://maplibre.org/) 6 and Vite, with
no UI framework.

```bash
npm ci
npm run dev        # http://localhost:5173, proxies /api and /tiles to a server on :8080 (BC_BACKEND to change)
npm run build      # type-checks, validates the styles, builds app + docs into ../server/internal/webui/dist
```

| Path | Contents |
|---|---|
| `src/app.ts` | Wires the map, panels, search, inspector and tools together |
| `src/map/style/` | Style builder: palettes (light/dark), layers, expressions |
| `src/map/controller.ts` | MapLibre wrapper: themes, global-state filters, overlays, selection, picking |
| `src/ui/` | Panels (explore, layers, transit, bridges, buildings), search, inspector, measuring tool |
| `src/api/` | Typed, dependency-free API client (reusable in other projects) |
| `build/` | Vite plugins: docs site, OpenAPI reference, style emission, Maki icons, precompression |
| `public/fonts/` | Noto Sans glyphs (re-fetch with `node build/fetch-fonts.mjs`) |

See [docs/styles.md](../docs/styles.md) for theming.
