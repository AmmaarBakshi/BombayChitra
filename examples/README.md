# Examples

Small, runnable programs that use BombayChitra from different languages. Start a server first
(`bombaychitra serve`, see [Getting started](../docs/getting-started.md)); every example reads the server origin
from `BOMBAYCHITRA_URL` (default `http://localhost:8080`).

| Language | File | What it does | Run |
|---|---|---|---|
| Shell | [`curl/examples.sh`](curl/examples.sh) | A tour of the API with curl | `sh curl/examples.sh` |
| Python | [`python/nearby.py`](python/nearby.py) | Nearest hospitals to a point (standard library only) | `python python/nearby.py` |
| Python | [`python/directions.py`](python/directions.py) | A trip by car, two-wheeler and on foot, with walking directions | `python python/directions.py` |
| Python | [`python/meeting_point.py`](python/meeting_point.py) | The fairest place for three friends to meet (distance matrix) and how far each gets in 20 minutes (isochrones) | `python python/meeting_point.py` |
| Python | [`python/offline_sqlite.py`](python/offline_sqlite.py) | Query `bombaychitra.db` directly, no server | `python python/offline_sqlite.py data/build/bombaychitra.db` |
| JavaScript | [`javascript/search.mjs`](javascript/search.mjs) | Search and reverse geocoding (Node 18+) | `node javascript/search.mjs "gateway of india"` |
| JavaScript | [`javascript/journey.mjs`](javascript/journey.mjs) | A train and metro journey, leg by leg (Node 18+) | `node javascript/journey.mjs 19.2290,72.8566 19.1860,72.9750` |
| HTML | [`html/embed.html`](html/embed.html) | Embed the map in any web page | open in a browser |
| Go | [`go/main.go`](go/main.go) | Metro and suburban lines with colours and lengths | `go run go/main.go` |
| Java | [`java/Bridges.java`](java/Bridges.java) | Longest named bridges (Java 21+) | `java java/Bridges.java` |
| C# | [`csharp/Buildings.cs`](csharp/Buildings.cs) | Tallest buildings (.NET 10 file-based app) | `dotnet run csharp/Buildings.cs` |
| PHP | [`php/atms.php`](php/atms.php) | ATMs in an area | `php php/atms.php thane` |
| Ruby | [`ruby/stations.rb`](ruby/stations.rb) | Metro stations and the lines serving them | `ruby ruby/stations.rb` |
| Rust | [`rust/`](rust/src/main.rs) | Area statistics | `cargo run --manifest-path rust/Cargo.toml -- makabo` |
| R | [`r/colleges.R`](r/colleges.R) | Colleges in MaKaBo as an `sf` data frame | `Rscript r/colleges.R` |
| SQL | [`sql/queries.sql`](sql/queries.sql) | Useful queries for the SQLite database | `sqlite3 data/build/bombaychitra.db < sql/queries.sql` |

Python and JavaScript also have client libraries in [`sdk/`](../sdk/); for other languages, generate a client from
`/openapi.json` — see [Integrations](../docs/integrations.md).
