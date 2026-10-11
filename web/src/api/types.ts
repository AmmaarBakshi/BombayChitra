// Response types of the BombayChitra REST API (/api/v1). Mirrors api/openapi.yaml.

export type LonLat = [number, number];
export type BBox = [number, number, number, number];

export interface Category {
  id: string;
  group: string;
  label: string;
  icon: string;
  rank: number;
  minzoom: number;
  count: number;
}

export interface CategoryGroup {
  id: string;
  label: string;
  colour: string;
  count: number;
  categories: Category[];
}

export interface AreaRef {
  id: string;
  name: string;
  kind?: string;
}

export interface Area extends AreaRef {
  short_name?: string;
  description?: string;
  area_km2: number;
  bbox: BBox;
  center: LonLat;
  osm_relations: number[];
  stats?: Record<string, number>;
  geometry?: GeoJSON.Geometry;
}

export interface Meta {
  name: string;
  server: string;
  region: { id: string; name: string; description: string; bounds: BBox; center: LonLat; zoom: number; maxzoom: number };
  dataset: { built_at: string; source: string; source_url: string; source_timestamp?: string; schema_version: string; generator: string };
  counts: Record<string, number>;
  attribution: string;
  license: string;
  links: Record<string, unknown>;
  areas: Area[];
}

export interface Place {
  id: number;
  osm_type: 'node' | 'way' | 'relation';
  osm_id: number;
  name?: string;
  category: string;
  group: string;
  lon: number;
  lat: number;
  locality?: string;
  distance_m?: number;
}

export interface LineRef {
  key: string;
  ref?: string;
  label: string;
  colour?: string;
  mode: string;
  /** `construction` or `proposed` for lines not yet in service. */
  state?: string;
}

export interface PlaceDetail extends Place {
  category_label: string;
  name_local?: string;
  address?: string;
  postcode?: string;
  phone?: string;
  website?: string;
  email?: string;
  opening_hours?: string;
  operator?: string;
  brand?: string;
  cuisine?: string;
  wheelchair?: string;
  wikidata?: string;
  wikipedia?: string;
  area_m2?: number;
  areas: AreaRef[];
  lines?: LineRef[];
  /** `overture` for places added from Overture Maps; their `osm_url` is empty. */
  source: 'openstreetmap' | 'overture';
  osm_url: string;
  tags: Record<string, string>;
}

export type SearchKind = 'area' | 'locality' | 'line' | 'place' | 'street' | 'bridge';

export interface SearchResult {
  kind: SearchKind;
  id: number | string;
  name: string;
  detail?: string;
  category?: string;
  lon: number;
  lat: number;
  bbox?: BBox;
  distance_m?: number;
}

export interface Locality {
  id: number;
  name: string;
  kind: string;
  lon: number;
  lat: number;
  population?: number;
  distance_m?: number;
}

export interface RoadMatch {
  id: number;
  street_id: number;
  name?: string;
  ref?: string;
  kind: string;
  distance_m: number;
  snapped: LonLat;
}

export interface Reverse {
  point: LonLat;
  label: string;
  road?: RoadMatch;
  locality?: Locality;
  areas: AreaRef[];
  nearby: Place[];
}

export interface Road {
  id: number;
  street_id: number;
  name?: string;
  ref?: string;
  kind: string;
  highway: string;
  oneway?: number;
  lanes?: number;
  maxspeed?: string;
  surface?: string;
  bridge: boolean;
  tunnel: boolean;
  length_m: number;
  osm_url: string;
  geometry?: GeoJSON.Geometry;
}

export interface Street {
  id: number;
  name?: string;
  ref?: string;
  kind: string;
  length_m: number;
  way_count: number;
  locality?: string;
  bbox: BBox;
  center: LonLat;
}

export interface StreetDetail extends Street {
  areas: AreaRef[];
  ways: Road[];
  geometry: GeoJSON.Geometry;
}

export interface Bridge {
  id: number;
  named_bridge_id?: number;
  name?: string;
  kind: string;
  class?: string;
  structure?: string;
  layer?: number;
  ref?: string;
  lanes?: number;
  length_m: number;
  area_m2?: number;
  lon: number;
  lat: number;
  osm_url: string;
  geometry?: GeoJSON.Geometry;
  tags?: Record<string, string>;
}

export interface NamedBridge {
  id: number;
  name: string;
  kind: string;
  length_m: number;
  total_length_m: number;
  way_count: number;
  locality?: string;
  bbox: BBox;
  center: LonLat;
  ways?: Bridge[];
  geometry?: GeoJSON.Geometry;
}

export interface Building {
  id: number;
  name?: string;
  kind?: string;
  levels?: number;
  height_m?: number;
  min_height_m?: number;
  footprint_m2: number;
  lon: number;
  lat: number;
  address?: string;
  locality?: string;
  osm_url: string;
  tags?: Record<string, string>;
}

export interface TransitStop {
  id: number;
  name?: string;
  lon: number;
  lat: number;
}

export interface TransitRoute {
  /** OSM id of the route relation, or of the longest track way for lines assembled from tracks. */
  id: number;
  osm_type: 'relation' | 'way';
  line_key: string;
  label: string;
  name?: string;
  mode: string;
  network?: string;
  ref?: string;
  colour?: string;
  operator?: string;
  from?: string;
  to?: string;
  state?: string;
  length_m: number;
  stop_count: number;
  bbox: BBox;
  stops?: TransitStop[];
  geometry?: GeoJSON.Geometry;
  osm_url: string;
}

export interface RouteEnd {
  lat: number;
  lon: number;
  /** The nearest point on a drivable road, where the route starts or ends. */
  snapped: { lat: number; lon: number; road?: string; offset_m: number };
}

export interface RouteResult {
  optimize: 'time' | 'distance';
  distance_m: number;
  duration_s: number;
  straight_m: number;
  from: RouteEnd;
  to: RouteEnd;
  /** Roads in driving order; unnamed stretches have no name. */
  roads: { name?: string; distance_m: number }[];
  bbox: BBox;
  geometry: GeoJSON.LineString;
}

export interface TransitLine {
  key: string;
  label: string;
  mode: string;
  network?: string;
  ref?: string;
  colour?: string;
  length_m: number;
  bbox: BBox;
  routes: TransitRoute[];
}

export interface ListResponse<T> {
  count: number;
  items: T[];
}

export interface Stats {
  area: string;
  totals: Record<string, number>;
  places_by_category: Record<string, number>;
}
