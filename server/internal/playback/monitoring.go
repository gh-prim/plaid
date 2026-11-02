package playback

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

// Monitoring is the admin's view of a period of plays: the data sent each
// day and by whom, where remote plays came from, and how well each device
// plays files as they are.
type Monitoring struct {
	Days    []DayBytes
	People  []PersonBytes
	Places  []PlacePlays
	Devices []DeviceHealth
	// Local and Remote count plays; Unplaced, remote plays without a place.
	Local, Remote, Unplaced int
}

// DayBytes is the data sent one local day.
type DayBytes struct {
	Date  string
	Bytes int64
}

// PersonBytes is one account's share of the period, most data first.
type PersonBytes struct {
	UserID, DisplayName string
	Bytes               int64
	Plays               int
	WatchedMs           int64
}

// PlacePlays is a city remote plays came from.
type PlacePlays struct {
	Country, City string
	Lat, Lon      float64
	Plays, People int
	Last          time.Time
}

// DeviceHealth scores a device: plays sent as is, transcodes, and plays
// its codecs couldn't take.
type DeviceHealth struct {
	DeviceName, DisplayName                     string
	Plays, DirectPlay, DirectStream, Transcoded int
	// CodecIssues counts plays converted because of a codec or format.
	CodecIssues int
	Last        time.Time
	// Score is 0–100: half the direct play rate, 30 for few transcodes,
	// 20 for few codec problems.
	Score int
}

// codecReasons are the reasons that say a device can't decode something.
var codecReasons = map[string]bool{
	ReasonContainer: true, ReasonVideoCodec: true, ReasonVideoProfile: true, ReasonBitDepth: true,
	ReasonHDR: true, ReasonAudioCodec: true, ReasonAudioChannels: true,
}

// Monitor sums up the last days of plays (today last). offset is the
// browser's getTimezoneOffset, in minutes.
func (s *Service) Monitor(ctx context.Context, days, offset int) (Monitoring, error) {
	loc := time.FixedZone("", -offset*60)
	now := s.o.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -(days - 1))
	out := Monitoring{Days: make([]DayBytes, days)}
	index := map[string]int{}
	for i := range out.Days {
		d := from.AddDate(0, 0, i).Format("2006-01-02")
		out.Days[i] = DayBytes{Date: d}
		index[d] = i
	}
	const layout = "2006-01-02T15:04:05.000Z"
	rows, err := s.o.DB.QueryContext(ctx, `SELECT COALESCE(user_id, ''), display_name, device_name, method, remote,
		started_at, watched_ms, COALESCE(bitrate_bps, 0), COALESCE(reasons, ''), COALESCE(country, ''), COALESCE(city, ''),
		lat, lon FROM play_log WHERE started_at >= ? ORDER BY started_at`, from.UTC().Format(layout))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	people := map[string]*PersonBytes{}
	places := map[string]*PlacePlays{}
	placePeople := map[string]map[string]bool{}
	devices := map[string]*DeviceHealth{}
	for rows.Next() {
		var (
			userID, display, device, method, started, reasons, country, city string
			remote                                                           bool
			watched, bitrate                                                 int64
			lat, lon                                                         *float64
		)
		if err := rows.Scan(&userID, &display, &device, &method, &remote, &started, &watched, &bitrate, &reasons,
			&country, &city, &lat, &lon); err != nil {
			return out, err
		}
		at, _ := time.Parse(layout, started)
		bytes := bitrate * watched / 8000
		if i, ok := index[at.In(loc).Format("2006-01-02")]; ok {
			out.Days[i].Bytes += bytes
		}
		who := userID + "\x00" + display
		p := people[who]
		if p == nil {
			p = &PersonBytes{UserID: userID, DisplayName: display}
			people[who] = p
		}
		p.Bytes += bytes
		p.Plays++
		p.WatchedMs += watched

		if !remote {
			out.Local++
		} else {
			out.Remote++
			if lat == nil || lon == nil {
				out.Unplaced++
			} else {
				key := country + "\x00" + city
				pl := places[key]
				if pl == nil {
					pl = &PlacePlays{Country: country, City: city, Lat: *lat, Lon: *lon}
					places[key] = pl
					placePeople[key] = map[string]bool{}
				}
				pl.Plays++
				pl.Last = at
				placePeople[key][who] = true
			}
		}

		dkey := who + "\x00" + device
		d := devices[dkey]
		if d == nil {
			d = &DeviceHealth{DeviceName: device, DisplayName: display}
			devices[dkey] = d
		}
		d.Plays++
		d.Last = at
		switch method {
		case DirectPlay:
			d.DirectPlay++
		case DirectStream:
			d.DirectStream++
		case Transcode:
			d.Transcoded++
		}
		for _, r := range strings.Split(reasons, ",") {
			if codecReasons[r] {
				d.CodecIssues++
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	for _, p := range people {
		out.People = append(out.People, *p)
	}
	sort.Slice(out.People, func(i, j int) bool {
		a, b := out.People[i], out.People[j]
		return a.Bytes > b.Bytes || a.Bytes == b.Bytes && a.WatchedMs > b.WatchedMs
	})
	for key, pl := range places {
		pl.People = len(placePeople[key])
		out.Places = append(out.Places, *pl)
	}
	sort.Slice(out.Places, func(i, j int) bool {
		a, b := out.Places[i], out.Places[j]
		return a.Plays > b.Plays || a.Plays == b.Plays && a.City < b.City
	})
	for _, d := range devices {
		d.Score = healthScore(*d)
		out.Devices = append(out.Devices, *d)
	}
	// Worst first: the devices to look at.
	sort.Slice(out.Devices, func(i, j int) bool {
		a, b := out.Devices[i], out.Devices[j]
		return a.Score < b.Score || a.Score == b.Score && a.Plays > b.Plays
	})
	return out, nil
}

func healthScore(d DeviceHealth) int {
	if d.Plays == 0 {
		return 100
	}
	n := float64(d.Plays)
	score := 50*float64(d.DirectPlay)/n + 30*(1-float64(d.Transcoded)/n) + 20*(1-float64(d.CodecIssues)/n)
	return int(math.Round(score))
}

// GeoReady says whether remote plays can be placed on the map.
func (s *Service) GeoReady() bool {
	r, ok := s.o.Geo.(interface{ Ready() bool })
	return ok && r.Ready()
}
