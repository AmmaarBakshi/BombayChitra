# The web app

Open the server in a browser (for example `http://localhost:8080`) to use the map. Everything below works in English,
Marathi and Hindi — the **EN / मरा / हिं** button in the toolbar switches the interface and the map labels.

## Find things

- **Search** (press `/`) finds areas, localities, stations and lines, places, streets and bridges, however their
  names are spelled ("Chatrapati", "Vileparle"), and PIN codes such as `400050`.
- **Explore** picks an area — a city, MaKaBo, a BMC ward — with its statistics, and finds hospitals, schools, banks
  and more in view.
- **Near me** in the Explore panel lists the three nearest **stations**, **hospitals** or **ATMs** from where you are
  (the browser asks for your location; without it, from the centre of the map), with the distance and walking time.
  Each opens its card, and **Directions** plans the way there: a walk within 2 km, otherwise by train and metro.
- **Right-click** (or long-press) anywhere for "What's here?": the road, locality, PIN code and places around.

## Get from A to B

The **Distance** panel plans between two places chosen by search, on the map, or with "Route here" on any card:

| Travel by | What you get |
|---|---|
| **Car**, **Two-wheeler** | Distance and time on the roads, fastest or shortest, with or without tolls, turn-by-turn directions and warnings about monsoon waterlogging hotspots in season. Two-wheelers stay off expressways and sea links. |
| **Walk** | Streets, footpaths, steps and foot over-bridges at 4.8 km/h. |
| **Train & metro** | The quickest journey on the suburban railway, the metro and the monorail: the walk to the station, each train with its line, direction and stations, any change of line, and the walk at the end. The **Local**, **Metro** and **Monorail** switches choose what to ride. |

Journey times are estimates from the usual gap between trains and typical running times — not a timetable — so allow
for crowding and the last train at night. See [Train, metro and monorail journeys](api.md#train-metro-and-monorail-journeys)
for how they are worked out.

## Install it and use it offline

- **Install:** in Chrome, Edge and other Chromium browsers an **install** button appears in the toolbar (also in the
  browser's menu); on an iPhone use Share → Add to Home Screen. The installed app opens in its own window.
- **Offline:** after the first visit the app itself opens without a connection. To keep the **map** of a place, zoom
  to it and choose **Layers → Offline maps → Save this area**: the vector tiles of the area in view are saved in the
  browser, from a few zoom levels below the current one down to street level (up to 4,000 tiles; a neighbourhood
  takes a few megabytes). Saved areas are listed with their size, can be shown or deleted, and are drawn whenever the
  network is unavailable. Search, routes and cards still need the server.

## Keyboard

| Key | Action |
|---|---|
| `/` | Search |
| `↑` `↓` `Enter` | Choose a search result |
| `Esc` | Close the card or the search results |
| `H` | Hide or show the dashboard for a full-screen map |
| `←` `→` `Home` `End` | Move between the panel tabs |
| Arrow keys, `+` `−` | Pan and zoom the map (after "Skip to the map") |

## Shareable links

The address bar always describes what you see, so a link reopens the same view:

| Parameter | Meaning |
|---|---|
| `map=15/18.9322/72.8347` | Zoom, latitude, longitude (and bearing, pitch) |
| `area=makabo` | The selected area |
| `sel=place:1074415162` | The open card |
| `dist=19.0190,72.8430~19.0864,72.9087` | A trip from one point to another (`lat,lon`) |
| `mode=foot`, `mode=two_wheeler`, `mode=transit` | How to travel (car when absent) |
| `opt=distance`, `avoid=toll` | Shortest instead of fastest; keep off toll roads |
| `ride=metro` | For journeys, what to ride: `train`, `metro`, `monorail` |
