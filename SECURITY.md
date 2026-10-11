# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub security advisories](https://github.com/AmmaarBakshi/BombayChitra/security/advisories/new), not in public
issues. Include steps to reproduce and the affected version (`bombaychitra version`). You will get an acknowledgement
within a few days.

## Scope and design notes

- The server is read-only: it serves a SQLite database opened with `mode=ro&immutable=1` and a tile archive, and
  exposes only `GET`, `HEAD` and `OPTIONS`.
- All SQL is parameterised; full-text queries are built from tokenised user input, never interpolated.
- Data from OpenStreetMap is rendered in the web app with DOM text nodes (never `innerHTML`); website links are
  forced to `http(s)`.
- The server does not rate-limit; put it behind a reverse proxy or CDN when exposing it publicly
  (see [Deployment](docs/deployment.md)).
