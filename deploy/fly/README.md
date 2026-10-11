# Deploying a public demo on Fly.io

The cheapest way to put BombayChitra online: one shared-CPU machine with 512 MB of memory in Mumbai (`bom`), the
dataset baked into the image and machines that stop when idle.

```bash
fly auth login
cd deploy/fly
fly launch --copy-config --no-deploy --name bombaychitra-demo   # pick your own app name
fly deploy                                                       # builds the image, downloads the data (~300 MB)
fly certs add maps.example.com                                   # optional custom domain
```

What the configuration does:

- **Image** — [`Dockerfile`](Dockerfile) starts from the published `ghcr.io/ammaarbakshi/bombaychitra` image and
  runs `bombaychitra download`, which fetches `bombaychitra.pmtiles` and `bombaychitra.db` from the `data-latest`
  release and checks them against its `SHA256SUMS`. Pin versions with build arguments:
  `fly deploy --build-arg BC_VERSION=0.1.0 --build-arg BC_DATA_RELEASE=v0.1.0`.
- **Abuse protection** — `BC_RATE_LIMIT=300` and `BC_RATE_BURST=60` per client; `BC_TRUST_PROXY=true` so the
  limit applies to the real client address Fly's proxy reports, not the proxy.
- **Health** — Fly checks `/api/v1/health`, which fails when the database cannot be read.
- **Metrics** — Prometheus metrics on the private port 9091, scraped by Fly's managed Prometheus and visible in
  the Fly dashboard (Grafana); they are not reachable from the internet.
- **Cost** — with `auto_stop_machines` the machine sleeps when nobody uses it and wakes on the next request.

## Updating the data

The **Data** workflow publishes a fresh `data-latest` every Monday. Redeploy to bake it in:

```bash
fly deploy --no-cache
```

To automate this, add a scheduled GitHub Actions job that runs `flyctl deploy --remote-only --no-cache` with a
`FLY_API_TOKEN` secret after the Data workflow.
