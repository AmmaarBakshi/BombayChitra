# Getting started

There are three ways to run BombayChitra: **from source** (recommended for development), with **Docker**, or with a
**prebuilt release** binary plus the published data.

## Option 1 — from source

### Requirements

| Tool | Version | Used for |
|---|---|---|
| Java (JDK) | 21 or newer | Building the data (Planetiler) |
| Go | 1.26 or newer | The server |
| Node.js | 22 or newer | Building the web app and docs |
| Disk / RAM | ~1.5 GB free disk, 8 GB RAM (the build uses a 4 GB heap) | The OpenStreetMap extract and build |

### 1. Clone

```bash
git clone https://github.com/AmmaarBakshi/BombayChitra.git
cd BombayChitra
```

### 2. Build the data

This downloads the OpenStreetMap extract for western India (~220 MB, once), then builds
`data/build/bombaychitra.pmtiles` and `data/build/bombaychitra.db`. It takes about a minute on an 8-core machine.

```bash
./scripts/build-data.sh          # macOS, Linux, Git Bash, WSL
```

```powershell
.\scripts\build-data.ps1         # Windows PowerShell
```

### 3. Build the web app and the server

```bash
cd web && npm ci && npm run build && cd ..
cd server && go build -o ../bin/bombaychitra ./cmd/bombaychitra && cd ..
```

The web build writes into `server/internal/webui/dist`, which the server embeds, so the result is **one binary**
containing the map, the docs, the styles and the API.

### 4. Run

```bash
./bin/bombaychitra serve
```

Open **http://localhost:8080**. The API is at `/api/v1`, the docs at `/docs/`.

With `make` the same steps are `make data`, `make all` and `make run`.

## Option 2 — Docker

```bash
docker compose run --rm pipeline     # builds ./data/build (first run downloads OSM)
docker compose up -d server          # http://localhost:8080
```

## Option 3 — prebuilt release

1. Download the binary for your platform from the
   [releases page](https://github.com/AmmaarBakshi/BombayChitra/releases).
2. Download `bombaychitra.pmtiles` and `bombaychitra.db` from the **Data (latest)** release into `data/build/`.
3. Run `bombaychitra serve`.

The data release is rebuilt weekly from the latest OpenStreetMap data by a GitHub Actions workflow.

## Developing the web app

Run the server with your data, then start Vite's dev server, which proxies `/api` and `/tiles` to it:

```bash
./bin/bombaychitra serve &        # :8080
cd web && npm run dev             # http://localhost:5173 with hot reload
```

Point the dev server at another backend with `BC_BACKEND=http://host:port npm run dev`.

## Next steps

- [Architecture](architecture.md) — how the pieces fit together.
- [API guide](api.md) — query the data from your own code.
- [Configuration](configuration.md) — change the region or add place categories.
