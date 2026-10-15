"""Where should three friends meet? Travel times from each home to a few candidate places (a distance matrix), and how
far each can get in 20 minutes (isochrones) — using only the standard library.

    python meeting_point.py
"""
import json
import os
import urllib.parse
import urllib.request

BASE = os.environ.get("BOMBAYCHITRA_URL", "http://localhost:8080") + "/api/v1"

HOMES = {"Andheri": (19.1197, 72.8464), "Bandra": (19.0544, 72.8402), "Ghatkopar": (19.0864, 72.9087)}
PLACES = {"BKC": (19.0660, 72.8680), "Dadar": (19.0190, 72.8430), "Powai": (19.1176, 72.9060)}


def call(path: str, body: dict | None = None, **query) -> dict:
    url = f"{BASE}{path}" + (f"?{urllib.parse.urlencode(query)}" if query else "")
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"} if data else {})
    with urllib.request.urlopen(req, timeout=60) as res:
        return json.load(res)


def main() -> None:
    point = lambda p: {"lat": p[0], "lon": p[1]}  # noqa: E731
    matrix = call("/matrix", {"sources": [point(p) for p in HOMES.values()],
                              "destinations": [point(p) for p in PLACES.values()], "mode": "two_wheeler"})
    print(f"{'By two-wheeler (min)':22}" + "".join(f"{name:>10}" for name in PLACES))
    for home, row in zip(HOMES, matrix["durations"]):
        print(f"{home:22}" + "".join(f"{'—' if s is None else round(s / 60):>10}" for s in row))
    # The fairest place is the one where the longest trip is shortest.
    worst = [max(row[j] or float("inf") for row in matrix["durations"]) for j in range(len(PLACES))]
    best = min(range(len(PLACES)), key=worst.__getitem__)
    print(f"\nMeet at {list(PLACES)[best]}: nobody travels more than {round(worst[best] / 60)} min.\n")

    for home, (lat, lon) in HOMES.items():
        iso = call("/isochrone", **{"from": f"{lat},{lon}", "mode": "two_wheeler", "minutes": "20"})
        area = iso["features"][0]["properties"]["area_km2"]
        print(f"In 20 minutes from {home} you can reach {area:.0f} sq km of the city.")


if __name__ == "__main__":
    main()
