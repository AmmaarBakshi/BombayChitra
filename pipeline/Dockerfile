# syntax=docker/dockerfile:1.7
# BombayChitra data pipeline: OpenStreetMap + Overture Maps → bombaychitra.pmtiles + bombaychitra.db.
#   docker run --rm -v "$PWD/data:/work" bombaychitra-pipeline
#   docker run --rm -e SKIP_OVERTURE=1 -v "$PWD/data:/work" bombaychitra-pipeline   (OpenStreetMap only)

FROM eclipse-temurin:25-jdk AS build
ARG PLANETILER_VERSION=0.10.2
ARG PLANETILER_SHA256=f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d
ADD --checksum=sha256:${PLANETILER_SHA256} \
    https://github.com/onthegomap/planetiler/releases/download/v${PLANETILER_VERSION}/planetiler.jar /opt/planetiler.jar
COPY pipeline/src /src
RUN mkdir /classes && javac -encoding UTF-8 -implicit:none -sourcepath /src -cp /opt/planetiler.jar -d /classes $(find /src -name '*.java')

FROM eclipse-temurin:25-jre
ARG OVERTURE_CLI_VERSION=1.0.2
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-venv \
 && python3 -m venv /opt/overture && /opt/overture/bin/pip install --no-cache-dir "overturemaps==${OVERTURE_CLI_VERSION}" \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/planetiler.jar /opt/planetiler.jar
COPY --from=build /classes /opt/pipeline
COPY config /opt/config
COPY scripts/fetch-overture.py /opt/fetch-overture.py
COPY --chmod=755 pipeline/docker-entrypoint.sh /opt/entrypoint.sh
WORKDIR /work
ENV JAVA_TOOL_OPTIONS="-Xmx4g"
ENTRYPOINT ["/opt/entrypoint.sh"]
CMD ["--download"]
