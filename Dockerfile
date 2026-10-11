# syntax=docker/dockerfile:1
# BombayChitra server: one static binary with the web app, docs and styles embedded.
# Data is mounted at /data (bombaychitra.pmtiles + bombaychitra.db).

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY config /src/config
COPY docs /src/docs
COPY api /src/api
COPY web /src/web
RUN mkdir -p /src/server/internal/webui && npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server ./
COPY --from=web /src/server/internal/webui/dist ./internal/webui/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/AmmaarBakshi/BombayChitra/server/internal/httpapi.Version=${VERSION}" \
    -o /out/bombaychitra ./cmd/bombaychitra

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="BombayChitra" \
      org.opencontainers.image.description="Open, detailed map of the Mumbai Metropolitan Region" \
      org.opencontainers.image.source="https://github.com/AmmaarBakshi/BombayChitra" \
      org.opencontainers.image.licenses="MIT"
COPY --from=server /out/bombaychitra /usr/local/bin/bombaychitra
ENV BC_ADDR=:8080 BC_DATA=/data BC_LOG_FORMAT=json
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/usr/local/bin/bombaychitra"]
CMD ["serve"]
