# Deployment

BombayChitra is one binary plus two data files, so deployment is simple. Anything that can run a Linux binary or
a container will do.

## Docker

```bash
docker compose run --rm pipeline          # build ./data/build once (or download the data release)
docker compose up -d server               # serves on :8080
```

Or run the published image with your data mounted:

```bash
docker run -d --name bombaychitra -p 8080:8080 \
  -v "$PWD/data/build:/data:ro" \
  -e BC_PUBLIC_URL=https://maps.example.com \
  ghcr.io/ammaarbakshi/bombaychitra:latest
```

The image is distroless, runs as a non-root user, and is about 20 MB.

## systemd

```ini
# /etc/systemd/system/bombaychitra.service
[Unit]
Description=BombayChitra map server
After=network-online.target

[Service]
ExecStart=/usr/local/bin/bombaychitra serve --addr 127.0.0.1:8080 --data /srv/bombaychitra
Environment=BC_PUBLIC_URL=https://maps.example.com BC_LOG_FORMAT=json
DynamicUser=yes
ReadOnlyPaths=/srv/bombaychitra
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Behind a reverse proxy

Terminate TLS in the proxy and either set `BC_PUBLIC_URL` or pass forwarding headers with `--trust-proxy`.

**Caddy:**

```text
maps.example.com {
  encode zstd gzip
  reverse_proxy 127.0.0.1:8080
}
```

**Nginx:**

```text
server {
  server_name maps.example.com;
  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host $host;
  }
}
```

Do not let the proxy re-compress `/tiles/` responses: they are already gzip-encoded.

## CDN

Responses carry strong caching headers, so a CDN in front of the server absorbs most traffic:

| Path | Suggested edge TTL |
|---|---|
| `/assets/*`, `/fonts/*` | 1 year (content-hashed) / 30 days |
| `/tiles/*` | 1 day; purge after a data update |
| `/api/v1/*` | 5 minutes |
| `/`, `/docs/*`, `/styles/*` | revalidate (ETag) |

## Serverless / static hosting

For a map without the API, host the web build and `bombaychitra.pmtiles` on any static host that supports range
requests (S3, Cloudflare R2, GitHub Pages) and load the archive with the `pmtiles` protocol — see
[Vector tiles](tiles.md#reading-the-archive-directly).

## Updating data

1. Rebuild with `scripts/build-data.sh --refresh_sources` (or download the latest data release).
2. Replace `bombaychitra.pmtiles` and `bombaychitra.db` together.
3. Restart the server and, if you use a CDN, purge `/tiles/*` and `/api/*`.

The **Data** GitHub Actions workflow rebuilds the dataset every Monday from the latest OpenStreetMap extract and
publishes it to the `data-latest` release, so a deployment can be refreshed with a scheduled download.

## Sizing

The server is I/O-light: tile requests are a single read from a 43 MB file that fits in the page cache, and most API
queries are index lookups. A single small VM (1 vCPU, 512 MB RAM) comfortably serves a city-scale audience,
especially behind a CDN.
