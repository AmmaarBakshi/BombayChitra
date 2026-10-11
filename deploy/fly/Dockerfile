# syntax=docker/dockerfile:1
# Fly.io image for a public BombayChitra: the published server image with the dataset baked in, so a machine starts
# in a couple of seconds without a volume. Redeploy after a data release to pick up new data (see README.md).
ARG BC_VERSION=latest
FROM ghcr.io/ammaarbakshi/bombaychitra:${BC_VERSION}
# Build arguments are environment variables while RUN executes; the download command reads them.
ARG BC_DATA_RELEASE=data-latest
ENV BC_DATA=/home/nonroot/data
# The image is distroless (no shell): the server binary downloads and verifies the data itself.
RUN ["/usr/local/bin/bombaychitra", "download"]
