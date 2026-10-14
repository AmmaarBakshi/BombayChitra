package journey

// RouteOf returns the route a ride is on and whether it is an express pattern (calling at far fewer stations than
// the line's stopping trains).
func (n *Network) RouteOf(l Leg) (Route, bool) {
	r := &n.routes[l.Route]
	return r.Route, r.express
}

// Via lists the stations a ride calls at between boarding and getting off.
func (n *Network) Via(l Leg) []int {
	r := &n.routes[l.Route]
	out := []int{}
	for i := l.FromStop + 1; i < l.ToStop; i++ {
		out = append(out, r.station[i])
	}
	return out
}

// RideGeometry is the line a ride follows: its track between the two stations when the route has one, otherwise
// the stops in order.
func (n *Network) RideGeometry(l Leg) [][2]float64 {
	r := &n.routes[l.Route]
	if r.track == nil {
		out := make([][2]float64, 0, l.ToStop-l.FromStop+1)
		for i := l.FromStop; i <= l.ToStop; i++ {
			out = append(out, [2]float64{r.Stops[i].Lon, r.Stops[i].Lat})
		}
		return out
	}
	a, b := r.at[l.FromStop], r.at[l.ToStop]
	out := [][2]float64{interpolate(r.track, r.trackAt, a)}
	for i, c := range r.track {
		if r.trackAt[i] > a && r.trackAt[i] < b {
			out = append(out, c)
		}
	}
	return append(out, interpolate(r.track, r.trackAt, b))
}

// Stats describes the network's size.
type Stats struct {
	Lines    int `json:"lines"`
	Routes   int `json:"routes"`
	Stations int `json:"stations"`
}

// Stats returns the number of lines, routes and stations.
func (n *Network) Stats() Stats {
	return Stats{Lines: len(n.Lines), Routes: len(n.routes), Stations: len(n.Stations)}
}

// interpolate returns the point d metres along line (with cumulative lengths cum).
func interpolate(line [][2]float64, cum []float64, d float64) [2]float64 {
	for i := 1; i < len(line); i++ {
		if cum[i] >= d {
			t := 0.0
			if seg := cum[i] - cum[i-1]; seg > 0 {
				t = (d - cum[i-1]) / seg
			}
			return [2]float64{line[i-1][0] + t*(line[i][0]-line[i-1][0]), line[i-1][1] + t*(line[i][1]-line[i-1][1])}
		}
	}
	return line[len(line)-1]
}
