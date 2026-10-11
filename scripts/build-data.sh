#!/usr/bin/env bash
# Builds the BombayChitra dataset from OpenStreetMap:
#   data/build/bombaychitra.pmtiles  vector tiles
#   data/build/bombaychitra.db       SQLite database (places, roads, bridges, buildings, transit, search)
#
# Requirements: Java 21+ (JDK), curl. Extra arguments are passed to the pipeline, e.g.
#   scripts/build-data.sh --threads=4 --only_layers=roads,pois
#   JAVA_OPTS=-Xmx8g scripts/build-data.sh --refresh_sources
set -euo pipefail
cd "$(dirname "$0")/.."

PLANETILER_VERSION="${PLANETILER_VERSION:-0.10.2}"
PLANETILER_SHA256="${PLANETILER_SHA256:-f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d}"
JAVA_OPTS="${JAVA_OPTS:--Xmx4g}"
JAR="data/tools/planetiler-${PLANETILER_VERSION}.jar"
CLASSES="pipeline/build/classes"

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: '$1' is required" >&2; exit 1; }; }
need java
need javac
need curl

java_major=$(java -XshowSettings:properties -version 2>&1 | awk -F'= ' '/java.specification.version/ {print $2}' | cut -d. -f1)
if [ "${java_major:-0}" -lt 21 ]; then
  echo "error: Java 21 or newer is required (found ${java_major:-unknown})" >&2
  exit 1
fi

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

mkdir -p data/tools data/sources data/build
if [ ! -f "$JAR" ]; then
  echo "→ downloading Planetiler ${PLANETILER_VERSION}"
  curl -fL --retry 3 -o "$JAR.part" "https://github.com/onthegomap/planetiler/releases/download/v${PLANETILER_VERSION}/planetiler.jar"
  actual=$(sha256 "$JAR.part")
  if [ "$actual" != "$PLANETILER_SHA256" ]; then
    rm -f "$JAR.part"
    echo "error: checksum mismatch for planetiler.jar ($actual)" >&2
    exit 1
  fi
  mv "$JAR.part" "$JAR"
fi

echo "→ compiling pipeline"
rm -rf "$CLASSES"
mkdir -p "$CLASSES"
# shellcheck disable=SC2046
javac -encoding UTF-8 -implicit:none -sourcepath pipeline/src -cp "$JAR" -d "$CLASSES" $(find pipeline/src -name '*.java')

sep=":"
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) sep=";" ;; esac

echo "→ building data (first run downloads the OpenStreetMap extract)"
# shellcheck disable=SC2086
java $JAVA_OPTS --enable-native-access=ALL-UNNAMED -cp "${JAR}${sep}${CLASSES}" bombaychitra.pipeline.Main --download "$@"

echo "✓ done:"
ls -lh data/build/bombaychitra.pmtiles data/build/bombaychitra.db
