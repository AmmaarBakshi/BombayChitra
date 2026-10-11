# Deployment

BombayChitra is one binary plus two data files, so deployment is simple. Anything that can run a Linux binary or
a container will do. Ready-made configurations are in [`deploy/`](../deploy):

| | For | |
|---|---|---|
| [`deploy/fly`](../deploy/fly) | A public demo on Fly.io: one 512 MB machine in Mumbai, data baked into the image | `fly deploy` |
| [`deploy/compose`](../deploy/compose) | Any VPS with Docker: the server behind Caddy with automatic HTTPS | `DOMAIN=… docker compose up -d` |
| [`deploy/systemd`](../deploy/systemd) | A plain Linux server: hardened service and a weekly data refresh timer | `systemctl enable --now …` |

## Getting the data

You do not need Java or a pipeline build to run a server. The server downloads the published dataset itself:

```bash
bombaychitra download                    # → data/build, from the weekly data-latest release
bombaychitra download --release v0.1.0   # the dataset a version was released with
bombaychitra serve
```

`download` checks both files against the release's `SHA256SUMS`, skips files that are already current, and only
replaces the old pair once both new files have arrived intact — safe to run from a cron job or timer while the
server is running (restart it afterwards to serve the new data).

## Public servers

Before putting a server on the internet:

1. **Turn on rate limiting** — `--rate-limit 300 --rate-burst 60` is a comfortable per-client allowance for people
   using the map; add `--api-keys` for your own apps. See [rate limiting](server.md#rate-limiting-and-api-keys).
2. **Pass `--trust-proxy`** when a reverse proxy or load balancer is in front, so limits apply per client and
   URLs in styles use the public host and scheme.
3. **Keep metrics private** with `--metrics-addr 127.0.0.1:9091` (or `--metrics=false`).
4. **Probe `/api/v1/health`** from your load balancer or uptime monitor.
5. **Refresh the data weekly** — the `data-latest` release is rebuilt every Monday.

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

The image is distroless, runs as a non-root user, and is about 20 MB. It has no shell, but the binary can fetch
data into a volume by itself:

```bash
docker run --rm -u 0 -v bombaychitra-data:/data -e BC_DATA=/data ghcr.io/ammaarbakshi/bombaychitra download
docker run -d -p 8080:8080 -v bombaychitra-data:/data:ro -e BC_DATA=/data ghcr.io/ammaarbakshi/bombaychitra
```

[`deploy/compose`](../deploy/compose) wraps this up with Caddy for HTTPS.

## systemd

[`deploy/systemd`](../deploy/systemd) has a hardened service (read-only file system, no capabilities, private
`/tmp`) that downloads the dataset on its first start, and a timer that refreshes it every Monday:

```bash
sudo useradd --system --home-dir /var/lib/bombaychitra --shell /usr/sbin/nologin bombaychitra
sudo install -m 0755 bombaychitra /usr/local/bin/
sudo cp deploy/systemd/bombaychitra*.service deploy/systemd/bombaychitra-update.timer /etc/systemd/system/
sudo systemctl enable --now bombaychitra.service bombaychitra-update.timer
```

Override settings such as `BC_PUBLIC_URL` or `BC_API_KEYS` in `/etc/bombaychitra/env`.

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

1. Run `bombaychitra download` (or rebuild with `scripts/build-data.sh --refresh_sources`).
2. `bombaychitra.pmtiles` and `bombaychitra.db` must always be replaced together; `download` does this for you.
3. Restart the server and, if you use a CDN, purge `/tiles/*` and `/api/*`.

The **Data** GitHub Actions workflow rebuilds the dataset every Monday from the latest OpenStreetMap extract and
publishes it to the `data-latest` release with a `SHA256SUMS` file, so a deployment can be refreshed with a
scheduled `bombaychitra download` — the systemd timer in [`deploy/systemd`](../deploy/systemd) does exactly that.

## Sizing

The server is I/O-light: tile requests are a single read from a ~96 MB file that fits in the page cache, and most API
queries are index lookups. A single small VM (1 vCPU, 512 MB RAM) comfortably serves a city-scale audience,
especially behind a CDN.
