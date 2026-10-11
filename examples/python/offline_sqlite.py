"""Query the BombayChitra SQLite database directly — no server needed.

    python offline_sqlite.py [path/to/bombaychitra.db]
"""
import math
import sqlite3
import sys

path = sys.argv[1] if len(sys.argv) > 1 else "data/build/bombaychitra.db"
db = sqlite3.connect(f"file:{path}?mode=ro", uri=True)

print("Places per area:")
for name, count in db.execute(
    "SELECT a.name, s.value FROM area_stats s JOIN areas a ON a.id = s.area_id "
    "WHERE s.key = 'places' ORDER BY s.value DESC"
):
    print(f"  {name:<28} {int(count):>7,}")

print("\nSearch for 'siddhivinayak':")
for kind, name, detail in db.execute(
    """SELECT kind, name, detail FROM search WHERE search MATCH '"siddhivinayak"*'
       ORDER BY bm25(search, 10.0, 4.0, 1.0) * (1 - 0.15 * weight) LIMIT 5"""
):
    print(f"  [{kind}] {name} — {detail}")


def metres(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    dlat, dlon = math.radians(lat2 - lat1), math.radians(lon2 - lon1)
    h = math.sin(dlat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(dlon / 2) ** 2
    return 2 * 6371008.8 * math.asin(math.sqrt(h))


print("\nCafés within 500 m of Churchgate station (R*Tree filter + exact distance):")
lat, lon, box = 18.9353, 72.8272, 0.0055
rows = db.execute(
    """SELECT p.name, p.lat, p.lon FROM places_rtree r JOIN places p ON p.id = r.id
       WHERE r.min_lon >= ? AND r.max_lon <= ? AND r.min_lat >= ? AND r.max_lat <= ?
         AND p.category = 'cafe' AND p.name IS NOT NULL""",
    (lon - box, lon + box, lat - box, lat + box),
)
nearby = sorted((metres(lat, lon, plat, plon), name) for name, plat, plon in rows)
for d, name in nearby:
    if d <= 500:
        print(f"  {d:>4.0f} m  {name}")
