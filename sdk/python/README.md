# bombaychitra (Python)

A small, dependency-free client for the [BombayChitra](https://github.com/AmmaarBakshi/BombayChitra) API — the open
map of Mumbai, Thane, Navi Mumbai and the wider MMR. It is generated from `api/openapi.yaml`, so every endpoint is a
method named after its operation.

```bash
pip install bombaychitra        # or: pip install ./sdk/python from a checkout
```

```python
from bombaychitra import BombayChitra, BombayChitraError

api = BombayChitra("http://localhost:8080")        # api_key="…" for servers that require one

csmt = api.search("CST", limit=1)["items"][0]
print(csmt["name"], csmt["lat"], csmt["lon"])

route = api.route("19.1197,72.8464", "18.9400,72.8355", mode="two_wheeler")
print(route["distance_m"], route["duration_s"])

journey = api.journey("19.0190,72.8430", "19.0864,72.9087", modes="train,metro")
for leg in journey["legs"]:
    print(leg["type"], leg.get("line", {}).get("label", ""), leg["duration_s"])

matrix = api.matrix({"sources": [{"lat": 19.1197, "lon": 72.8464}, {"lat": 18.94, "lon": 72.8355}]})
print(matrix["durations"])

try:
    api.route("19.1197,72.8464", "18.5,72.0")
except BombayChitraError as err:
    print(err.status, err.code, err.message)
```

Required parameters are positional (path ids, `q`, `from`/`to` — `from_` in Python), the rest are keyword
arguments; lists can be given as Python lists. Responses are the API's JSON as dictionaries and lists, GeoJSON
included. See the [API guide](https://github.com/AmmaarBakshi/BombayChitra/blob/main/docs/api.md) for every field.
