#!/usr/bin/env bash
# Prints the CHANGELOG.md section of a version (without its heading), e.g. for release notes:
#   scripts/changelog-section.sh 0.1.0
#   scripts/changelog-section.sh Unreleased
set -euo pipefail
version="${1:?usage: changelog-section.sh <version>}"
version="${version#v}"
file="${2:-$(dirname "$0")/../CHANGELOG.md}"
awk -v want="$version" '
  /^## \[/ {
    if (found) exit
    heading = $0
    sub(/^## \[/, "", heading)
    sub(/\].*$/, "", heading)
    if (heading == want) { found = 1; next }
  }
  /^\[[^]]+\]: / && found { exit }
  found { print }
' "$file" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
