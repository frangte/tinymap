// Package weather is Saigon's weather now, for the game (docs/design/README.md, D24): fetched from Open-Meteo every
// 15 minutes off the world loop (Source), and sent by a world module to every player on arrival and whenever it
// changes (Module), so everyone in the city sees the same rain. Without a fresh report the sky is clear.
package weather

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Kind is the weather in one word, as the game draws it.
type Kind string

const (
	Clear   Kind = "clear"
	Cloudy  Kind = "cloudy"
	Drizzle Kind = "drizzle"
	Rain    Kind = "rain"
	Storm   Kind = "storm"
)

// Now is the weather the game draws: its kind, how heavy the rain is and how much of the sky is cloud (0–1), the
// temperature, and when it was observed.
type Now struct {
	Kind  Kind
	Rain  float64
	Cloud float64
	Temp  float64
	At    time.Time
}

// ClearSky is what the game shows without a report.
var ClearSky = Now{Kind: Clear}

// FromWMO reads a WMO weather code (as Open-Meteo gives it) into a kind and a rain strength. Saigon has no snow:
// snow codes count as rain.
func FromWMO(code int) (Kind, float64) {
	switch code {
	case 0, 1, 2:
		return Clear, 0
	case 3, 45, 48: // overcast, fog
		return Cloudy, 0
	case 51, 56: // light drizzle
		return Drizzle, 0.2
	case 53, 55, 57:
		return Drizzle, 0.35
	case 61, 66, 80: // slight rain, slight showers
		return Rain, 0.5
	case 63, 81, 71, 73, 77, 85:
		return Rain, 0.75
	case 65, 67, 82, 75, 86:
		return Rain, 1
	case 95, 96, 99: // thunderstorm
		return Storm, 1
	}
	return Clear, 0
}

// report is the part of Open-Meteo's forecast answer the game uses (`current=`, see Source).
type report struct {
	Current struct {
		Time   string   `json:"time"`
		Code   *int     `json:"weather_code"`
		Cloud  *float64 `json:"cloud_cover"`
		Precip *float64 `json:"precipitation"`
		Temp   *float64 `json:"temperature_2m"`
	} `json:"current"`
	UTCOffset int `json:"utc_offset_seconds"`
}

// parse reads Open-Meteo's answer. A report without a weather code is an error; rain falling under a code that
// says dry (the model's precipitation leads its code now and then) is drizzle.
func parse(body []byte) (Now, error) {
	var r report
	if err := json.Unmarshal(body, &r); err != nil {
		return Now{}, fmt.Errorf("weather report: %w", err)
	}
	c := r.Current
	if c.Code == nil {
		return Now{}, fmt.Errorf("weather report: no weather_code")
	}
	kind, rain := FromWMO(*c.Code)
	now := Now{Kind: kind, Rain: rain}
	if c.Precip != nil && *c.Precip > 0 && (kind == Clear || kind == Cloudy) {
		now.Kind, now.Rain = Drizzle, 0.2
	}
	if c.Cloud != nil {
		now.Cloud = clamp01(*c.Cloud / 100)
	}
	// overcast, rain and storms come under a grey sky, whatever share of it the model calls cloud
	if now.Kind != Clear {
		now.Cloud = math.Max(now.Cloud, 0.7)
	}
	if c.Temp != nil {
		now.Temp = math.Round(*c.Temp*10) / 10
	}
	// Open-Meteo's time is local to the timezone asked for (Asia/Ho_Chi_Minh), without an offset
	if at, err := time.Parse("2006-01-02T15:04", c.Time); err == nil {
		now.At = at.Add(-time.Duration(r.UTCOffset) * time.Second)
	}
	return now, nil
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
