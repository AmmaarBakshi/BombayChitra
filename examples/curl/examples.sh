#!/bin/sh
# A quick tour of the BombayChitra API.
B="${BOMBAYCHITRA_URL:-http://localhost:8080}/api/v1"
set -x
curl -s "$B/meta"
curl -s "$B/search?q=andheri%20station&limit=3"
curl -s "$B/reverse?lat=19.0545&lon=72.8405"
curl -s "$B/nearby?lat=19.0178&lon=72.8478&radius=800&category=atm,bank&limit=5"
curl -s "$B/places?area=makabo&category=college&named=true&limit=5"
curl -s "$B/bridges/named?kind=road&limit=5"
curl -s "$B/buildings?sort=height&limit=5"
curl -s "$B/transit/lines?mode=subway"
curl -s "$B/route?from=19.1197,72.8464&to=19.0660,72.8680"
curl -s "$B/route?from=19.1197,72.8464&to=19.0660,72.8680&mode=foot"
curl -s "$B/route?from=18.9985,72.8580&to=18.9310,73.0300&mode=car&avoid=toll&steps=false"
curl -s "$B/stats?area=navi-mumbai"
