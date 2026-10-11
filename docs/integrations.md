# Integrations

BombayChitra speaks open standards, so it plugs into almost any stack. There are three integration surfaces:

| Surface | Use it for | Works with |
|---|---|---|
| **Vector tiles + styles** | Showing the map | MapLibre GL JS, MapLibre Native (Android, iOS), Flutter, React Native, Leaflet, QGIS, OpenLayers |
| **REST API** | Search, reverse geocoding, places, roads, bridges, buildings, transit | Any language with HTTP + JSON |
| **Data files** | Offline, analytics, embedding in apps | Any SQLite driver; PMTiles readers; GeoJSON/CSV exports |

Replace `https://maps.example.com` below with your server (for example `http://localhost:8080`). Complete,
runnable versions of these snippets are in [`examples/`](../examples/README.md).

## Show the map

### Any web page (no build step)

```html
<div id="map" style="height: 480px"></div>
<link rel="stylesheet" href="https://unpkg.com/maplibre-gl@6.13.0/dist/maplibre-gl.css" />
<script type="module">
  import * as maplibregl from 'https://unpkg.com/maplibre-gl@6.13.0/dist/maplibre-gl.mjs';

  new maplibregl.Map({
    container: 'map',
    style: 'https://maps.example.com/styles/light.json', // or dark.json
    center: [72.8777, 19.076],
    zoom: 11,
  });
</script>
```

Or embed the full app: `<iframe src="https://maps.example.com/#map=13/19.06/72.84" width="100%" height="480"></iframe>`.

### Leaflet

```js
// npm i leaflet maplibre-gl @maplibre/maplibre-gl-leaflet
const map = L.map('map').setView([19.076, 72.8777], 12);
L.maplibreGL({ style: 'https://maps.example.com/styles/light.json' }).addTo(map);
```

### Android (Kotlin, MapLibre Native)

```kotlin
MapLibre.getInstance(this)
setContentView(R.layout.activity_map)
val mapView = findViewById<MapView>(R.id.mapView)
mapView.getMapAsync { map ->
    map.setStyle(Style.Builder().fromUri("https://maps.example.com/styles/light.json"))
    map.cameraPosition = CameraPosition.Builder().target(LatLng(19.076, 72.8777)).zoom(11.0).build()
}
```

### iOS (Swift, MapLibre Native)

```swift
import MapLibre

let mapView = MLNMapView(frame: view.bounds, styleURL: URL(string: "https://maps.example.com/styles/dark.json"))
mapView.setCenter(CLLocationCoordinate2D(latitude: 19.076, longitude: 72.8777), zoomLevel: 11, animated: false)
view.addSubview(mapView)
```

### Flutter

```dart
// pubspec: maplibre_gl
MapLibreMap(
  styleString: 'https://maps.example.com/styles/light.json',
  initialCameraPosition: const CameraPosition(target: LatLng(19.076, 72.8777), zoom: 11),
)
```

### React Native

```tsx
// npm i @maplibre/maplibre-react-native
<MapView style={{ flex: 1 }} mapStyle="https://maps.example.com/styles/light.json" />
```

### QGIS

*Layer → Add Layer → Add Vector Tile Layer → New → Generic connection*:

- URL: `https://maps.example.com/tiles/{z}/{x}/{y}.mvt`, max zoom `15`
- Style URL: `https://maps.example.com/styles/light.json`

For analysis, export layers to GeoJSON with `bombaychitra export` and add them as vector layers.

## Call the API

### Python

```python
import requests

BASE = "https://maps.example.com/api/v1"
res = requests.get(f"{BASE}/nearby", params={"lat": 19.0178, "lon": 72.8478, "radius": 1000, "category": "hospital"})
res.raise_for_status()
for p in res.json()["items"]:
    print(f'{p.get("name", "Hospital"):45} {p["distance_m"]:>6.0f} m')
```

With GeoPandas: `geopandas.read_file(f"{BASE}/places?category=college&area=makabo&format=geojson")`.

### JavaScript / TypeScript

```js
const res = await fetch('https://maps.example.com/api/v1/search?q=gateway%20of%20india&limit=5');
const { items } = await res.json();
console.log(items.map((r) => `${r.name} — ${r.detail}`));
```

The typed client in `web/src/api/client.ts` has no dependencies and works in browsers and Node 18+:

```ts
import { BombayChitraClient } from './client';
const api = new BombayChitraClient('https://maps.example.com');
const { items } = await api.namedBridges({ kind: ['road'], limit: 5 });
```

### Go

```go
resp, err := http.Get("https://maps.example.com/api/v1/transit/lines?mode=subway")
if err != nil {
	log.Fatal(err)
}
defer resp.Body.Close()
var out struct {
	Items []struct {
		Label   string  `json:"label"`
		Colour  string  `json:"colour"`
		LengthM float64 `json:"length_m"`
	} `json:"items"`
}
json.NewDecoder(resp.Body).Decode(&out)
for _, l := range out.Items {
	fmt.Printf("%-12s %s %.1f km\n", l.Label, l.Colour, l.LengthM/1000)
}
```

### Java

```java
var client = HttpClient.newHttpClient();
var request = HttpRequest.newBuilder(URI.create("https://maps.example.com/api/v1/reverse?lat=19.0545&lon=72.8405")).build();
String json = client.send(request, HttpResponse.BodyHandlers.ofString()).body();
```

### C#

```csharp
using var http = new HttpClient { BaseAddress = new Uri("https://maps.example.com/api/v1/") };
var doc = await http.GetFromJsonAsync<JsonElement>("bridges/named?kind=road&limit=5");
foreach (var b in doc.GetProperty("items").EnumerateArray())
    Console.WriteLine($"{b.GetProperty("name")}: {b.GetProperty("length_m")} m");
```

### PHP

```php
$data = json_decode(file_get_contents('https://maps.example.com/api/v1/places?category=atm&area=thane&limit=10'), true);
foreach ($data['items'] as $p) {
    echo ($p['name'] ?? 'ATM') . " ({$p['lat']}, {$p['lon']})\n";
}
```

### Ruby

```ruby
require 'net/http'
require 'json'

data = JSON.parse(Net::HTTP.get(URI('https://maps.example.com/api/v1/buildings?sort=height&limit=5')))
data['items'].each { |b| puts "#{b['name']}: #{b['height_m']} m" }
```

### Rust

```rust
let stats: serde_json::Value = reqwest::blocking::get("https://maps.example.com/api/v1/stats?area=makabo")?.json()?;
println!("places in MaKaBo: {}", stats["totals"]["places"]);
```

### R

```r
library(sf)
colleges <- st_read("https://maps.example.com/api/v1/places?category=college&limit=500&format=geojson")
```

### Any other language

Generate a typed client from the OpenAPI document:

```bash
npx @openapitools/openapi-generator-cli generate -i https://maps.example.com/openapi.json -g <language> -o out/
```

There are generators for Kotlin, Swift, Dart, C++, Elixir, Scala, Haskell and dozens more.

## Use the data files

The SQLite database can ship inside a mobile app for offline search, feed a data warehouse, or be queried from a
notebook. See [Database](database.md) for the schema and queries. The PMTiles archive can be hosted on any static
storage and read directly by MapLibre — see [Vector tiles](tiles.md).
