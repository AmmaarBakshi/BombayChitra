# syntax=docker/dockerfile:1.7
# BombayChitra data pipeline: OpenStreetMap → bombaychitra.pmtiles + bombaychitra.db.
#   docker run --rm -v "$PWD/data:/work" bombaychitra-pipeline

FROM eclipse-temurin:25-jdk AS build
ARG PLANETILER_VERSION=0.10.2
ARG PLANETILER_SHA256=f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d
ADD --checksum=sha256:${PLANETILER_SHA256} \
    https://github.com/onthegomap/planetiler/releases/download/v${PLANETILER_VERSION}/planetiler.jar /opt/planetiler.jar
COPY pipeline/src /src
RUN mkdir /classes && javac -encoding UTF-8 -implicit:none -sourcepath /src -cp /opt/planetiler.jar -d /classes $(find /src -name '*.java')

FROM eclipse-temurin:25-jre
COPY --from=build /opt/planetiler.jar /opt/planetiler.jar
COPY --from=build /classes /opt/pipeline
COPY config /opt/config
WORKDIR /work
ENV JAVA_TOOL_OPTIONS="-Xmx4g"
ENTRYPOINT ["java", "--enable-native-access=ALL-UNNAMED", "-cp", "/opt/planetiler.jar:/opt/pipeline", \
            "bombaychitra.pipeline.Main", "--config_dir=/opt/config", "--data_dir=/work"]
CMD ["--download"]
