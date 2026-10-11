#!/usr/bin/env python3
"""Downloads the Overture Maps extracts configured in config/region.json ("overture" section).

Places and buildings are cut to the region's bounds as GeoParquet under data/sources/, using the official
`overturemaps` downloader (run inside the virtual environment the build scripts create). Existing files are kept
unless --refresh is given. The pipeline skips any file that is missing, so a failed download never breaks a build.

    python scripts/fetch-overture.py [--refresh] [--config config/region.json] [--data data]
"""
import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

TYPES = {"places": "place", "buildings": "building"}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--config", default="config/region.json")
    parser.add_argument("--data", default="data")
    parser.add_argument("--refresh", action="store_true", help="download again even if the files exist")
    args = parser.parse_args()

    region = json.loads(Path(args.config).read_text(encoding="utf-8"))
    overture = region.get("overture")
    if not overture:
        print("Overture Maps is not configured in region.json; nothing to download")
        return 0
    bbox = ",".join(str(v) for v in region["bounds"])
    release = overture.get("release", "latest")
    cli = shutil.which("overturemaps", path=os.path.dirname(sys.executable)) or shutil.which("overturemaps")
    if not cli:
        print("warning: the overturemaps downloader is not installed; skipping Overture Maps", file=sys.stderr)
        return 0

    failed = False
    for key, kind in TYPES.items():
        file = (overture.get(key) or {}).get("file")
        if not file:
            continue
        target = Path(args.data) / "sources" / file
        if target.exists() and not args.refresh:
            print(f"-> Overture {key}: {target} exists (use --refresh_sources to update)")
            continue
        target.parent.mkdir(parents=True, exist_ok=True)
        part = target.with_name(target.name + ".part")
        command = [cli, "download", f"--bbox={bbox}", "-f", "geoparquet", "-t", kind, "-o", str(part)]
        if release != "latest":
            command += ["-r", release]
        print(f"-> downloading Overture {key} for {bbox} ({release})")
        if subprocess.run(command).returncode != 0 or not part.exists():
            print(f"warning: Overture {key} download failed; the build continues without it", file=sys.stderr)
            part.unlink(missing_ok=True)
            failed = True
            continue
        part.replace(target)
        state = part.with_name(part.name + ".state")
        if state.exists():
            state.replace(target.with_name(target.name + ".state"))
        print(f"   {target} ({target.stat().st_size / 1e6:.0f} MB)")
    return 1 if failed and os.environ.get("OVERTURE_REQUIRED") == "1" else 0


if __name__ == "__main__":
    sys.exit(main())
