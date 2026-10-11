"""Nearest hospitals to a point, using only the Python standard library.

    python nearby.py [lat lon]
"""
import json
import os
import sys
import urllib.parse
import urllib.request

BASE = os.environ.get("BOMBAYCHITRA_URL", "http://localhost:8080") + "/api/v1"


def get(path: str, **params) -> dict:
    url = f"{BASE}{path}?{urllib.parse.urlencode(params)}"
    with urllib.request.urlopen(url, timeout=10) as res:
        return json.load(res)


def main() -> None:
    lat, lon = (float(sys.argv[1]), float(sys.argv[2])) if len(sys.argv) == 3 else (19.0178, 72.8478)  # Dadar
    here = get("/reverse", lat=lat, lon=lon)
    print(f"Near {here['label']}\n")
    hospitals = get("/nearby", lat=lat, lon=lon, radius=2000, category="hospital", named="true", limit=10)
    for place in hospitals["items"]:
        detail = get(f"/places/{place['id']}")
        print(f"{place['distance_m']:>6.0f} m  {place['name']:<45} {detail.get('phone', '')}")


if __name__ == "__main__":
    main()
