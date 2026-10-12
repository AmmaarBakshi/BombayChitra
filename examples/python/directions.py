"""Compare a trip by car, two-wheeler and on foot, with turn-by-turn directions, using only the standard library.

    python directions.py                                   # Andheri station to BKC
    python directions.py 18.9985,72.8580 18.9310,73.0300   # Sewri to Ulwe, over Atal Setu by car
"""
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("BOMBAYCHITRA_URL", "http://localhost:8080") + "/api/v1"


def route(origin: str, destination: str, **params) -> dict:
    query = urllib.parse.urlencode({"from": origin, "to": destination, **params})
    try:
        with urllib.request.urlopen(f"{BASE}/route?{query}", timeout=30) as res:
            return json.load(res)
    except urllib.error.HTTPError as e:
        return json.load(e)  # {"error": {"code": "no_route", ...}}


def minutes(seconds: float) -> str:
    m = round(seconds / 60)
    return f"{m // 60} h {m % 60} min" if m >= 60 else f"{m} min"


def main() -> None:
    origin, destination = (sys.argv[1], sys.argv[2]) if len(sys.argv) == 3 else ("19.1197,72.8464", "19.0660,72.8680")
    print(f"{'Mode':<12} {'Distance':>9} {'Time':>10}  Toll road")
    for mode in ("car", "two_wheeler", "foot"):
        r = route(origin, destination, mode=mode, steps="false")
        if "error" in r:
            print(f"{mode:<12} {r['error']['message']}")
            continue
        toll = f"{r['toll_m'] / 1000:.1f} km" if r["toll_m"] else "-"
        print(f"{mode:<12} {r['distance_m'] / 1000:>7.1f} km {minutes(r['duration_s']):>10}  {toll}")

    walk = route(origin, destination, mode="foot")
    if "steps" in walk:
        print("\nOn foot:")
        for i, step in enumerate(walk["steps"], 1):
            distance = f"{step['distance_m']:.0f} m" if step["distance_m"] else ""
            print(f"{i:>3}. {step['instruction']:<55} {distance:>8}")


if __name__ == "__main__":
    main()
