package main

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
)

// The drive heading is a region from OSM Scout. A 2-minute speed average
// decides which level: the nearest place when the gap to the next place of
// that kind would take at least 30 seconds, otherwise the next coarser name
// in its admin chain. Each coarser step is treated as about three times the
// gap. The same name stays current, and the entrance card runs only when
// that name changes. The drive view does not keep a city label.

const (
	cityRadiusM = 30000
	cityMoveM   = 400
	cityRetry   = 30 * time.Second
	speedWindow = 2 * time.Minute
	placeDwell  = 30 * time.Second
	// placeSpan is how much farther the next admin level is assumed to reach.
	placeSpan = 3
)

var scoutOrigin = "http://127.0.0.1:8553"

// enableCityNet is false in tests so a Draw with a position does not dial Scout.
var enableCityNet = true

// lookupCityFn is the Scout query. Tests replace it.
var lookupCityFn = lookupCityScout

var cityHTTP = &http.Client{Timeout: 8 * time.Second}

type cityKind struct {
	typ string
	max float64
}

// max is how far that place node can sit from the car and still be the
// place we are in. A city centre is farther from the edge than a quarter.
var cityKinds = []cityKind{
	{"place_neighbourhood", 1500},
	{"place_quarter", 1500},
	{"place_suburb", 3000},
	{"place_hamlet", 2500},
	{"place_village", 6000},
	{"place_town", 15000},
	{"place_city", 30000},
	{"place_bigcity", 30000},
}

// placeHit is one nearby Scout object. next is the gap in meters from this
// object's node to the next one of the same kind. Zero means Scout had no
// second object inside the query radius.
type placeHit struct {
	name  string
	admin string
	dist  float64
	next  float64
	max   float64
}

type speedSample struct {
	at  time.Time
	kmh float64
}

func (s *Shell) noteCity(now time.Time, fix gps.Fix) {
	if !s.uiReady || !enableCityNet {
		return
	}
	s.mu.Lock()
	if fix.HasSpeed {
		s.pushSpeedLocked(now, fix.Kmh)
	}
	if !fix.HasPos || s.cityBusy {
		s.mu.Unlock()
		return
	}
	if s.cityQueried {
		moved := metersBetween(s.cityLat, s.cityLon, fix.Lat, fix.Lon)
		if moved < cityMoveM {
			if s.cityAnswered {
				s.mu.Unlock()
				return
			}
			if now.Sub(s.cityFailed) < cityRetry {
				s.mu.Unlock()
				return
			}
		}
	}
	s.cityBusy = true
	s.cityQueried = true
	s.cityLat, s.cityLon = fix.Lat, fix.Lon
	s.mu.Unlock()
	go s.fetchCity(fix.Lat, fix.Lon)
}

func (s *Shell) pushSpeedLocked(now time.Time, kmh int) {
	v := float64(kmh)
	if n := len(s.speedHist); n > 0 && now.Sub(s.speedHist[n-1].at) < time.Second {
		s.speedHist[n-1].kmh = v
		s.speedHist[n-1].at = now
	} else {
		s.speedHist = append(s.speedHist, speedSample{at: now, kmh: v})
	}
	cut := now.Add(-speedWindow)
	i := 0
	for i < len(s.speedHist) && s.speedHist[i].at.Before(cut) {
		i++
	}
	if i > 0 {
		s.speedHist = append([]speedSample(nil), s.speedHist[i:]...)
	}
	s.speedKmh = avgSpeed(s.speedHist)
}

func avgSpeed(hist []speedSample) float64 {
	if len(hist) == 0 {
		return 0
	}
	var sum float64
	for _, h := range hist {
		sum += h.kmh
	}
	return sum / float64(len(hist))
}

func (s *Shell) fetchCity(lat, lon float64) {
	s.logf("city: asking OSM Scout")
	hits, ok := lookupCityFn(lat, lon)
	if !ok {
		s.logf("city: OSM Scout did not answer")
	} else if near, have := nearestHit(hits); !have {
		s.logf("city: no place nearby")
	} else {
		s.logf("city: %s", near.name)
	}
	s.mu.Lock()
	s.cityBusy = false
	if ok {
		s.cityAnswered = true
		s.cityHits = hits
	} else {
		s.cityFailed = time.Now()
	}
	s.mu.Unlock()
}

func (s *Shell) applyCity(now time.Time) {
	if !s.uiReady {
		return
	}
	s.mu.Lock()
	hits := s.cityHits
	kmh := s.speedKmh
	s.mu.Unlock()
	name := placeTitle(pickPlace(hits, kmh))
	if name == "" || name == s.place {
		return
	}
	s.place = name
	if name != s.areaName {
		s.areaName = name
		s.areaAt = now
	}
}

// placeTitle is the place itself. Scout puts the parent after a comma
// because two hierarchy levels go into the title.
func placeTitle(title string) string {
	title = cleanPart(title)
	if title == "" {
		return ""
	}
	return fitLine(title, 36)
}

func cleanPart(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	if i := strings.Index(title, ","); i >= 0 {
		title = strings.TrimSpace(title[:i])
	}
	return title
}

func nearestHit(hits []placeHit) (placeHit, bool) {
	best := -1
	for i, h := range hits {
		if h.name == "" || h.dist < 0 || h.dist > h.max {
			continue
		}
		if best < 0 || h.dist < hits[best].dist {
			best = i
		}
	}
	if best < 0 {
		return placeHit{}, false
	}
	return hits[best], true
}

// pickPlace is the finest region on the nearest place's admin chain that
// the current speed would not leave within placeDwell. kmh is the recent
// average; below 1 km/h the finest name is used.
func pickPlace(hits []placeHit, kmh float64) string {
	h, ok := nearestHit(hits)
	if !ok {
		return ""
	}
	if name := regionForSpeed(h, kmh); name != "" {
		return name
	}
	var coarse string
	coarseDist := math.MaxFloat64
	for _, o := range hits {
		if o.name == "" || o.dist < 0 || o.dist > o.max {
			continue
		}
		if o.next > 0 && dwellSeconds(o.next, kmh) < placeDwell.Seconds() {
			continue
		}
		if o.dist < coarseDist {
			coarseDist = o.dist
			coarse = cleanPart(o.name)
		}
	}
	return coarse
}

func regionForSpeed(h placeHit, kmh float64) string {
	parts := adminParts(h.admin)
	name := cleanPart(h.name)
	if name != "" && (len(parts) == 0 || !strings.EqualFold(parts[0], name)) {
		parts = append([]string{name}, parts...)
	}
	if len(parts) == 0 {
		return ""
	}
	if h.next <= 0 || kmh < 1 {
		return parts[0]
	}
	span := h.next
	first := parts[0]
	var last string
	for _, p := range parts {
		if last != "" && strings.EqualFold(last, p) {
			continue
		}
		last = p
		if dwellSeconds(span, kmh) >= placeDwell.Seconds() {
			return p
		}
		span *= placeSpan
	}
	if last != "" && !strings.EqualFold(last, first) {
		return last
	}
	return ""
}

func dwellSeconds(distM, kmh float64) float64 {
	if kmh < 1 || distM <= 0 {
		return math.Inf(1)
	}
	return distM / (kmh / 3.6)
}

func adminParts(admin string) []string {
	var parts []string
	for _, p := range strings.Split(admin, ",") {
		p = cleanPart(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) >= 3 && !strings.Contains(parts[len(parts)-1], " ") {
		parts = parts[:len(parts)-1]
	}
	var out []string
	for _, p := range parts {
		if isAdminUnit(p) {
			continue
		}
		if len(out) > 0 && strings.EqualFold(out[len(out)-1], p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isAdminUnit(p string) bool {
	l := strings.ToLower(p)
	for _, w := range []string{
		"województwo", "wojewodztwo", "powiat", "gmina", "metropolia",
		"voivodeship", "county", "municipality", "province",
	} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func lookupCityScout(lat, lon float64) ([]placeHit, bool) {
	hits := make(chan placeHit, len(cityKinds))
	var wg sync.WaitGroup
	okN := 0
	var okMu sync.Mutex
	for _, k := range cityKinds {
		wg.Add(1)
		go func(k cityKind) {
			defer wg.Done()
			hit, ok := fetchGuide(lat, lon, k)
			if !ok {
				return
			}
			okMu.Lock()
			okN++
			okMu.Unlock()
			if hit.name == "" {
				return
			}
			hits <- hit
		}(k)
	}
	wg.Wait()
	close(hits)
	if okN == 0 {
		return nil, false
	}
	var out []placeHit
	for h := range hits {
		out = append(out, h)
	}
	return out, true
}

func fetchGuide(lat, lon float64, kind cityKind) (placeHit, bool) {
	resp, err := cityHTTP.Get(guideURL(lat, lon, kind.typ))
	if err != nil {
		return placeHit{}, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return placeHit{}, false
	}
	name, admin, dist, next, ok := parseGuide(body)
	if !ok {
		return placeHit{}, false
	}
	return placeHit{name: name, admin: admin, dist: dist, next: next, max: kind.max}, true
}

func guideURL(lat, lon float64, kind string) string {
	q := url.Values{}
	q.Set("radius", strconv.Itoa(cityRadiusM))
	q.Set("limit", "2")
	q.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("lng", strconv.FormatFloat(lon, 'f', 6, 64))
	q.Set("poitype", kind)
	return scoutOrigin + "/v1/guide?" + q.Encode()
}

func parseGuide(body []byte) (name, admin string, dist, next float64, ok bool) {
	var doc struct {
		Results []struct {
			Title    string  `json:"title"`
			Admin    string  `json:"admin_region"`
			Distance float64 `json:"distance"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", 0, 0, false
	}
	if len(doc.Results) == 0 {
		return "", "", 0, 0, true
	}
	r := doc.Results[0]
	gap := 0.0
	if len(doc.Results) > 1 && doc.Results[1].Distance > r.Distance {
		gap = doc.Results[1].Distance - r.Distance
	}
	return cleanPart(r.Title), r.Admin, r.Distance, gap, true
}

func metersBetween(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371000.0
	r := func(d float64) float64 { return d * math.Pi / 180 }
	φ1, φ2 := r(lat1), r(lat2)
	dφ := r(lat2 - lat1)
	dλ := r(lon2 - lon1)
	h := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return 2 * earth * math.Asin(math.Min(1, math.Sqrt(h)))
}
