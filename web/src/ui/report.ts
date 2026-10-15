// "Report a problem": a new OpenStreetMap note at a place. Notes are public and local mappers watch them, so a wrong
// name, a closed shop or a missing road gets fixed at the source — and in BombayChitra's next weekly build.

/** The page that opens a new OpenStreetMap note at a point, zoomed in so the note lands on the right building. */
export function reportUrl(lon: number, lat: number): string {
  const la = lat.toFixed(6);
  const lo = lon.toFixed(6);
  return `https://www.openstreetmap.org/note/new?lat=${la}&lon=${lo}#map=19/${la}/${lo}`;
}
