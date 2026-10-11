#!/bin/sh
# Downloads the Overture Maps extracts (unless SKIP_OVERTURE=1), then runs the pipeline with the given arguments.
set -e
refresh=""
for arg in "$@"; do [ "$arg" = "--refresh_sources" ] && refresh="--refresh"; done
if [ "${SKIP_OVERTURE:-0}" != "1" ]; then
  # shellcheck disable=SC2086
  /opt/overture/bin/python /opt/fetch-overture.py --config /opt/config/region.json --data /work $refresh
fi
exec java --enable-native-access=ALL-UNNAMED -cp /opt/planetiler.jar:/opt/pipeline bombaychitra.pipeline.Main \
  --config_dir=/opt/config --data_dir=/work "$@"
