package content

import (
	"strings"
	"testing"
)

func TestEmbeddedPlacesAreValid(t *testing.T) {
	places, err := Places()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Place, len(places))
	for _, p := range places {
		byID[p.ID] = p
	}
	// ids the design docs and the game rely on
	for _, id := range []string{"cho_ben_thanh", "nha_tho_duc_ba", "bung_binh_quach_thi_trang", "duong_ham_nghi", "day_nha_pho_le_lai", "bai_bien"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("place %q is missing", id)
		}
	}
	if coast := byID["bai_bien"]; len(coast.Pts) < 3 || coast.Kind != Area {
		t.Errorf("bai_bien should be the sea front polyline, got %+v", coast)
	}
	// every part of the map has a name for the HUD to fall back on
	for _, id := range []string{"quan_1", "cho_lon", "thu_thiem", "khanh_hoi", "vung_tau"} {
		if p, ok := byID[id]; !ok || p.Kind != District {
			t.Errorf("district %q is missing or not a district: %+v", id, p)
		}
	}
}

func TestParsePlacesRejects(t *testing.T) {
	const ok = `{"id":"cho","name":{"vi":"Chợ","en":"Market"},"note":{"vi":"","en":""},"kind":"landmark","pts":[[8,8]],"r":25}`
	cases := []struct {
		name string
		json string
		want string
	}{
		{"no places", `{"places":[]}`, "no places"},
		{"unknown field", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"area","pts":[[0,0]],"r":1,"radius":3}]}`, "unknown field"},
		{"bad id", `{"places":[{"id":"Chợ","name":{"vi":"A","en":"A"},"kind":"area","pts":[[0,0]],"r":1}]}`, "lower_snake_case"},
		{"duplicate id", `{"places":[` + ok + `,` + ok + `]}`, "duplicate"},
		{"no name", `{"places":[{"id":"a","name":{"vi":"","en":""},"kind":"area","pts":[[0,0]],"r":1}]}`, "no name"},
		{"no English name", `{"places":[{"id":"a","name":{"vi":"A"},"kind":"area","pts":[[0,0]],"r":1}]}`, "no name in Vietnamese and English"},
		{"note in one language", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"note":{"vi":"x"},"kind":"area","pts":[[0,0]],"r":1}]}`, "both Vietnamese and English"},
		{"unknown kind", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"cafe","pts":[[0,0]],"r":1}]}`, "unknown kind"},
		{"no points", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"area","pts":[],"r":1}]}`, "no points"},
		{"zero reach", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"area","pts":[[0,0]],"r":0}]}`, "reach"},
		{"3d point", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"area","pts":[[0,0,0]],"r":1}]}`, "[x, y]"},
		{"off the map", `{"places":[{"id":"a","name":{"vi":"A","en":"A"},"kind":"area","pts":[[0,9000]],"r":1}]}`, "within"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parsePlaces([]byte(c.json))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

// map.json: the island's frame, with the spawn and the middle inside it, and a sane sea.
func TestMapIsChecked(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if x, z := c.Map.ThreeSpawn(); x != c.Map.Spawn[0] || z != -c.Map.Spawn[1] {
		t.Fatalf("spawn in three.js terms = (%v, %v), map %v", x, z, c.Map.Spawn)
	}
	ok := `"spawn":[0,0],"sea_level":-0.45,"island_center":[0,0],"bounds":[-10,-10,10,10]`
	if err := parseMap([]byte("{"+ok+"}"), &Catalog{}); err != nil {
		t.Fatalf("a good map: %v", err)
	}
	cases := []struct{ name, json, want string }{
		{"bounds the wrong way", `{"spawn":[0,0],"sea_level":0,"island_center":[0,0],"bounds":[10,-10,-10,10]}`, "bounds"},
		{"bounds off the map", `{"spawn":[0,0],"sea_level":0,"island_center":[0,0],"bounds":[-900,-10,10,10]}`, "bounds"},
		{"spawn outside", `{"spawn":[50,0],"sea_level":0,"island_center":[0,0],"bounds":[-10,-10,10,10]}`, "spawn"},
		{"a 3d spawn", `{"spawn":[0,0,0],"sea_level":0,"island_center":[0,0],"bounds":[-10,-10,10,10]}`, "spawn"},
		{"the middle outside", `{"spawn":[0,0],"sea_level":0,"island_center":[0,50],"bounds":[-10,-10,10,10]}`, "island_center"},
		{"a sea in the sky", `{"spawn":[0,0],"sea_level":45,"island_center":[0,0],"bounds":[-10,-10,10,10]}`, "sea_level"},
		{"an unknown field", `{` + ok + `,"nav":"saigon_mini_nav.png"}`, "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := parseMap([]byte(c.json), &Catalog{})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}
