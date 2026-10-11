package store

import "encoding/json"

// CategoryGroup is a top-level POI group (Health, Education, Transport…).
type CategoryGroup struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Colour     string     `json:"colour"`
	Count      int64      `json:"count"`
	Categories []Category `json:"categories"`
}

// Category is one POI category (hospital, school, metro_station…).
type Category struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	Label   string `json:"label"`
	Icon    string `json:"icon"`
	Rank    int    `json:"rank"`
	MinZoom int    `json:"minzoom"`
	Count   int64  `json:"count"`
}

// Area is a configured named area (Mumbai City, MaKaBo, Navi Mumbai…).
type Area struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	ShortName   string             `json:"short_name,omitempty"`
	Kind        string             `json:"kind,omitempty"`
	Description string             `json:"description,omitempty"`
	AreaKm2     float64            `json:"area_km2"`
	BBox        []float64          `json:"bbox"`
	Center      [2]float64         `json:"center"`
	Relations   json.RawMessage    `json:"osm_relations"`
	Stats       map[string]float64 `json:"stats,omitempty"`
	Geometry    json.RawMessage    `json:"geometry,omitempty"`
}

// AreaRef is a compact reference to an area.
type AreaRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

// Place is a point of interest summary.
type Place struct {
	ID        int64    `json:"id"`
	OSMType   string   `json:"osm_type"`
	OSMID     int64    `json:"osm_id"`
	Name      string   `json:"name,omitempty"`
	Category  string   `json:"category"`
	Group     string   `json:"group"`
	Lon       float64  `json:"lon"`
	Lat       float64  `json:"lat"`
	Locality  string   `json:"locality,omitempty"`
	DistanceM *float64 `json:"distance_m,omitempty"`
}

// PlaceDetail is the full record of a place.
type PlaceDetail struct {
	Place
	CategoryLabel string          `json:"category_label"`
	NameLocal     string          `json:"name_local,omitempty"`
	Address       string          `json:"address,omitempty"`
	Postcode      string          `json:"postcode,omitempty"`
	Phone         string          `json:"phone,omitempty"`
	Website       string          `json:"website,omitempty"`
	Email         string          `json:"email,omitempty"`
	OpeningHours  string          `json:"opening_hours,omitempty"`
	Operator      string          `json:"operator,omitempty"`
	Brand         string          `json:"brand,omitempty"`
	Cuisine       string          `json:"cuisine,omitempty"`
	Wheelchair    string          `json:"wheelchair,omitempty"`
	Wikidata      string          `json:"wikidata,omitempty"`
	Wikipedia     string          `json:"wikipedia,omitempty"`
	AreaM2        *float64        `json:"area_m2,omitempty"`
	Areas         []AreaRef       `json:"areas"`
	Lines         json.RawMessage `json:"lines,omitempty"`
	// Source is "openstreetmap", or "overture" for places added from Overture Maps (osm_url is then empty).
	Source string          `json:"source"`
	OSMURL string          `json:"osm_url"`
	Tags   json.RawMessage `json:"tags"`
}

// SearchResult is one hit from the unified search index.
type SearchResult struct {
	Kind      string    `json:"kind"`
	ID        any       `json:"id"`
	Name      string    `json:"name"`
	Detail    string    `json:"detail,omitempty"`
	Category  string    `json:"category,omitempty"`
	Lon       float64   `json:"lon"`
	Lat       float64   `json:"lat"`
	BBox      []float64 `json:"bbox,omitempty"`
	DistanceM *float64  `json:"distance_m,omitempty"`
	score     float64
}

// Street is a named road assembled from one or more OSM ways.
type Street struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name,omitempty"`
	Ref      string     `json:"ref,omitempty"`
	Kind     string     `json:"kind"`
	LengthM  float64    `json:"length_m"`
	WayCount int        `json:"way_count"`
	Locality string     `json:"locality,omitempty"`
	BBox     []float64  `json:"bbox"`
	Center   [2]float64 `json:"center"`
}

// Road is one OSM way of the road network.
type Road struct {
	ID       int64           `json:"id"`
	StreetID int64           `json:"street_id"`
	Name     string          `json:"name,omitempty"`
	Ref      string          `json:"ref,omitempty"`
	Kind     string          `json:"kind"`
	Highway  string          `json:"highway"`
	Oneway   *int64          `json:"oneway,omitempty"`
	Lanes    *int64          `json:"lanes,omitempty"`
	MaxSpeed string          `json:"maxspeed,omitempty"`
	Surface  string          `json:"surface,omitempty"`
	Bridge   bool            `json:"bridge"`
	Tunnel   bool            `json:"tunnel"`
	LengthM  float64         `json:"length_m"`
	OSMURL   string          `json:"osm_url"`
	Geometry json.RawMessage `json:"geometry,omitempty"`
}

// StreetDetail is a street with its member ways and merged geometry.
type StreetDetail struct {
	Street
	Areas    []AreaRef       `json:"areas"`
	Ways     []Road          `json:"ways"`
	Geometry json.RawMessage `json:"geometry"`
}

// Bridge is one bridge way or bridge outline.
type Bridge struct {
	ID        int64           `json:"id"`
	GroupID   *int64          `json:"named_bridge_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Kind      string          `json:"kind"`
	Class     string          `json:"class,omitempty"`
	Structure string          `json:"structure,omitempty"`
	Layer     *int64          `json:"layer,omitempty"`
	Ref       string          `json:"ref,omitempty"`
	Lanes     *int64          `json:"lanes,omitempty"`
	LengthM   float64         `json:"length_m"`
	AreaM2    float64         `json:"area_m2,omitempty"`
	Lon       float64         `json:"lon"`
	Lat       float64         `json:"lat"`
	OSMURL    string          `json:"osm_url"`
	Geometry  json.RawMessage `json:"geometry,omitempty"`
	Tags      json.RawMessage `json:"tags,omitempty"`
}

// NamedBridge groups the ways of one named bridge (sea links, flyovers, viaducts).
type NamedBridge struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	LengthM      float64         `json:"length_m"`
	TotalLengthM float64         `json:"total_length_m"`
	WayCount     int             `json:"way_count"`
	Locality     string          `json:"locality,omitempty"`
	BBox         []float64       `json:"bbox"`
	Center       [2]float64      `json:"center"`
	Ways         []Bridge        `json:"ways,omitempty"`
	Geometry     json.RawMessage `json:"geometry,omitempty"`
}

// Building is a building with descriptive attributes.
type Building struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	Levels     *int64          `json:"levels,omitempty"`
	HeightM    *float64        `json:"height_m,omitempty"`
	MinHeightM *float64        `json:"min_height_m,omitempty"`
	AreaM2     float64         `json:"footprint_m2"`
	Lon        float64         `json:"lon"`
	Lat        float64         `json:"lat"`
	Address    string          `json:"address,omitempty"`
	Locality   string          `json:"locality,omitempty"`
	OSMURL     string          `json:"osm_url"`
	Tags       json.RawMessage `json:"tags,omitempty"`
}

// TransitLine groups the route relations (directions/variants) of one line.
type TransitLine struct {
	Key     string         `json:"key"`
	Label   string         `json:"label"`
	Mode    string         `json:"mode"`
	Network string         `json:"network,omitempty"`
	Ref     string         `json:"ref,omitempty"`
	Colour  string         `json:"colour,omitempty"`
	LengthM float64        `json:"length_m"`
	BBox    []float64      `json:"bbox"`
	Routes  []TransitRoute `json:"routes"`
}

// TransitRoute is one OSM route relation (a direction or service pattern of a line).
type TransitRoute struct {
	ID        int64           `json:"id"`
	OSMType   string          `json:"osm_type"`
	LineKey   string          `json:"line_key"`
	Label     string          `json:"label"`
	Name      string          `json:"name,omitempty"`
	Mode      string          `json:"mode"`
	Network   string          `json:"network,omitempty"`
	Ref       string          `json:"ref,omitempty"`
	Colour    string          `json:"colour,omitempty"`
	Operator  string          `json:"operator,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	State     string          `json:"state,omitempty"`
	LengthM   float64         `json:"length_m"`
	StopCount int             `json:"stop_count"`
	BBox      []float64       `json:"bbox"`
	Stops     json.RawMessage `json:"stops,omitempty"`
	Geometry  json.RawMessage `json:"geometry,omitempty"`
	OSMURL    string          `json:"osm_url"`
}

// Locality is a populated place (suburb, neighbourhood, village…).
type Locality struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Lon        float64  `json:"lon"`
	Lat        float64  `json:"lat"`
	Population *int64   `json:"population,omitempty"`
	DistanceM  *float64 `json:"distance_m,omitempty"`
}

// Reverse is the result of reverse geocoding a coordinate.
type Reverse struct {
	Point    [2]float64 `json:"point"`
	Label    string     `json:"label"`
	Road     *RoadMatch `json:"road,omitempty"`
	Locality *Locality  `json:"locality,omitempty"`
	Areas    []AreaRef  `json:"areas"`
	Nearby   []Place    `json:"nearby"`
}

// RoadMatch is the nearest named road to a coordinate.
type RoadMatch struct {
	ID        int64      `json:"id"`
	StreetID  int64      `json:"street_id"`
	Name      string     `json:"name,omitempty"`
	Ref       string     `json:"ref,omitempty"`
	Kind      string     `json:"kind"`
	DistanceM float64    `json:"distance_m"`
	Snapped   [2]float64 `json:"snapped"`
}
