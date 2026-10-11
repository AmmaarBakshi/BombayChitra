#!/usr/bin/env bash
# Compiles the data pipeline and runs its JUnit tests (pipeline/test) with the JUnit console launcher — no Maven or
# Gradle. Downloads Planetiler and JUnit into data/tools on first use (checksums verified). Extra arguments go to the
# launcher, e.g. scripts/test-pipeline.sh --select-class=bombaychitra.pipeline.TaxonomyTest
set -euo pipefail
cd "$(dirname "$0")/.."

PLANETILER_VERSION="${PLANETILER_VERSION:-0.10.2}"
PLANETILER_SHA256="${PLANETILER_SHA256:-f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d}"
JUNIT_VERSION="${JUNIT_VERSION:-6.0.1}"
JUNIT_SHA256="${JUNIT_SHA256:-3009120b7953bfe63add272e65b2bbeca0d41d0dfd8dea605201db15b640e0ff}"
PLANETILER="data/tools/planetiler-${PLANETILER_VERSION}.jar"
JUNIT="data/tools/junit-platform-console-standalone-${JUNIT_VERSION}.jar"
CLASSES="pipeline/build/classes"
TEST_CLASSES="pipeline/build/test-classes"

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

fetch() { # url file sha256
  [ -f "$2" ] && return 0
  echo "→ downloading $(basename "$2")"
  mkdir -p "$(dirname "$2")"
  curl -fL --retry 3 -o "$2.part" "$1"
  if [ "$(sha256 "$2.part")" != "$3" ]; then
    rm -f "$2.part"
    echo "error: checksum mismatch for $(basename "$2")" >&2
    exit 1
  fi
  mv "$2.part" "$2"
}

fetch "https://github.com/onthegomap/planetiler/releases/download/v${PLANETILER_VERSION}/planetiler.jar" "$PLANETILER" "$PLANETILER_SHA256"
fetch "https://repo1.maven.org/maven2/org/junit/platform/junit-platform-console-standalone/${JUNIT_VERSION}/junit-platform-console-standalone-${JUNIT_VERSION}.jar" "$JUNIT" "$JUNIT_SHA256"

sep=":"
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) sep=";" ;; esac

echo "→ compiling pipeline and tests"
rm -rf "$CLASSES" "$TEST_CLASSES"
mkdir -p "$CLASSES" "$TEST_CLASSES"
# shellcheck disable=SC2046
javac -encoding UTF-8 -implicit:none -sourcepath pipeline/src -cp "$PLANETILER" -d "$CLASSES" $(find pipeline/src -name '*.java')
# shellcheck disable=SC2046
javac -encoding UTF-8 -cp "${PLANETILER}${sep}${CLASSES}${sep}${JUNIT}" -d "$TEST_CLASSES" $(find pipeline/test -name '*.java')

echo "→ running tests"
java --enable-native-access=ALL-UNNAMED -jar "$JUNIT" execute --disable-banner --details=tree \
  --class-path "${PLANETILER}${sep}${CLASSES}${sep}${TEST_CLASSES}" --scan-class-path "$TEST_CLASSES" "$@"
