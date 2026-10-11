// Package export writes dataset subsets as GeoJSON, newline-delimited GeoJSON or CSV for use in other tools.
package export

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// Kinds that can be exported.
var Kinds = []string{"places", "buildings", "bridges", "streets", "lines"}

// Formats that can be written.
var Formats = []string{"geojson", "ndjson", "csv"}

// Options selects what to export.
type Options struct {
	Kind       string
	Format     string
	Categories []string
	Groups     []string
	Area       string
}

type feature struct {
	Type       string          `json:"type"`
	ID         any             `json:"id,omitempty"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties any             `json:"properties"`
}

// writer abstracts the three output formats over a stream of features.
type writer struct {
	format string
	out    *bufio.Writer
	csv    *csv.Writer
	count  int
}

func newWriter(format string, out io.Writer) *writer {
	w := &writer{format: format, out: bufio.NewWriterSize(out, 1<<16)}
	if format == "csv" {
		w.csv = csv.NewWriter(w.out)
	} else if format == "geojson" {
		w.out.WriteString(`{"type":"FeatureCollection","features":[`)
	}
	return w
}

func (w *writer) feature(f feature) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if w.format == "geojson" && w.count > 0 {
		w.out.WriteByte(',')
	}
	w.out.Write(b)
	if w.format == "ndjson" {
		w.out.WriteByte('\n')
	}
	w.count++
	return nil
}

func (w *writer) row(cols []string) error {
	w.count++
	return w.csv.Write(cols)
}

func (w *writer) close() error {
	if w.csv != nil {
		w.csv.Flush()
		if err := w.csv.Error(); err != nil {
			return err
		}
	}
	if w.format == "geojson" {
		w.out.WriteString("]}\n")
	}
	return w.out.Flush()
}

func point(lon, lat float64) json.RawMessage {
	return json.RawMessage(`{"type":"Point","coordinates":[` + strconv.FormatFloat(lon, 'f', -1, 64) + "," +
		strconv.FormatFloat(lat, 'f', -1, 64) + "]}")
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// Run writes the selected data to out and returns the number of records written.
func Run(ctx context.Context, st *store.Store, opt Options, out io.Writer) (int, error) {
	w := newWriter(opt.Format, out)
	var err error
	switch opt.Kind {
	case "places":
		err = places(ctx, st, opt, w)
	case "buildings":
		err = buildings(ctx, st, opt, w)
	case "bridges":
		err = bridges(ctx, st, opt, w)
	case "streets":
		err = streets(ctx, st, opt, w)
	case "lines":
		err = lines(ctx, st, opt, w)
	default:
		err = fmt.Errorf("unknown kind %q", opt.Kind)
	}
	if err != nil {
		return w.count, err
	}
	return w.count, w.close()
}

func places(ctx context.Context, st *store.Store, opt Options, w *writer) error {
	if w.csv != nil {
		w.csv.Write([]string{"id", "osm_type", "osm_id", "name", "category", "group", "lon", "lat", "locality",
			"address", "postcode", "phone", "website", "opening_hours", "operator", "brand"})
	}
	q := store.PlaceQuery{Categories: opt.Categories, Groups: opt.Groups, Area: opt.Area}
	return st.EachPlace(ctx, q, func(p *store.PlaceDetail) error {
		if w.csv != nil {
			return w.row([]string{strconv.FormatInt(p.ID, 10), p.OSMType, strconv.FormatInt(p.OSMID, 10), p.Name,
				p.Category, p.Group, ftoa(p.Lon), ftoa(p.Lat), p.Locality, p.Address, p.Postcode, p.Phone, p.Website,
				p.OpeningHours, p.Operator, p.Brand})
		}
		return w.feature(feature{Type: "Feature", ID: p.ID, Geometry: point(p.Lon, p.Lat), Properties: p})
	})
}

func buildings(ctx context.Context, st *store.Store, opt Options, w *writer) error {
	if w.csv != nil {
		w.csv.Write([]string{"id", "name", "kind", "levels", "height_m", "footprint_m2", "lon", "lat", "address", "locality"})
	}
	items, err := st.Buildings(ctx, store.BuildingQuery{Area: opt.Area, Limit: 10_000_000})
	if err != nil {
		return err
	}
	for _, b := range items {
		if w.csv != nil {
			levels, height := "", ""
			if b.Levels != nil {
				levels = strconv.FormatInt(*b.Levels, 10)
			}
			if b.HeightM != nil {
				height = ftoa(*b.HeightM)
			}
			if err := w.row([]string{strconv.FormatInt(b.ID, 10), b.Name, b.Kind, levels, height, ftoa(b.AreaM2),
				ftoa(b.Lon), ftoa(b.Lat), b.Address, b.Locality}); err != nil {
				return err
			}
			continue
		}
		if err := w.feature(feature{Type: "Feature", ID: b.ID, Geometry: point(b.Lon, b.Lat), Properties: b}); err != nil {
			return err
		}
	}
	return nil
}

func bridges(ctx context.Context, st *store.Store, opt Options, w *writer) error {
	if w.csv != nil {
		w.csv.Write([]string{"id", "name", "kind", "length_m", "total_length_m", "way_count", "locality", "lon", "lat"})
	}
	items, err := st.NamedBridges(ctx, store.BridgeQuery{Area: opt.Area, Limit: 1_000_000})
	if err != nil {
		return err
	}
	for _, n := range items {
		if w.csv != nil {
			if err := w.row([]string{strconv.FormatInt(n.ID, 10), n.Name, n.Kind, ftoa(n.LengthM), ftoa(n.TotalLengthM),
				strconv.Itoa(n.WayCount), n.Locality, ftoa(n.Center[0]), ftoa(n.Center[1])}); err != nil {
				return err
			}
			continue
		}
		full, err := st.NamedBridge(ctx, n.ID)
		if err != nil {
			return err
		}
		geometry := full.Geometry
		full.Geometry, full.Ways = nil, nil
		if err := w.feature(feature{Type: "Feature", ID: n.ID, Geometry: geometry, Properties: full}); err != nil {
			return err
		}
	}
	return nil
}

func streets(ctx context.Context, st *store.Store, opt Options, w *writer) error {
	if w.csv != nil {
		w.csv.Write([]string{"id", "name", "ref", "kind", "length_m", "way_count", "locality", "lon", "lat"})
	}
	items, err := st.Streets(ctx, store.StreetQuery{Area: opt.Area, Limit: 1_000_000})
	if err != nil {
		return err
	}
	for _, s := range items {
		if w.csv != nil {
			if err := w.row([]string{strconv.FormatInt(s.ID, 10), s.Name, s.Ref, s.Kind, ftoa(s.LengthM),
				strconv.Itoa(s.WayCount), s.Locality, ftoa(s.Center[0]), ftoa(s.Center[1])}); err != nil {
				return err
			}
			continue
		}
		full, err := st.Street(ctx, s.ID)
		if err != nil {
			return err
		}
		geometry := full.Geometry
		full.Geometry, full.Ways = nil, nil
		if err := w.feature(feature{Type: "Feature", ID: s.ID, Geometry: geometry, Properties: full}); err != nil {
			return err
		}
	}
	return nil
}

func lines(ctx context.Context, st *store.Store, _ Options, w *writer) error {
	if w.csv != nil {
		w.csv.Write([]string{"id", "line_key", "label", "mode", "network", "ref", "colour", "from", "to", "length_m", "stop_count"})
	}
	all, err := st.TransitLines(ctx, nil, "")
	if err != nil {
		return err
	}
	for _, line := range all {
		for _, r := range line.Routes {
			if w.csv != nil {
				if err := w.row([]string{strconv.FormatInt(r.ID, 10), r.LineKey, r.Label, r.Mode, r.Network, r.Ref,
					r.Colour, r.From, r.To, ftoa(r.LengthM), strconv.Itoa(r.StopCount)}); err != nil {
					return err
				}
				continue
			}
			full, err := st.TransitRoute(ctx, r.ID)
			if err != nil {
				return err
			}
			geometry := full.Geometry
			full.Geometry = nil
			if err := w.feature(feature{Type: "Feature", ID: r.ID, Geometry: geometry, Properties: full}); err != nil {
				return err
			}
		}
	}
	return nil
}
