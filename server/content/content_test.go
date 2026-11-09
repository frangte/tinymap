package content

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
		t.Errorf("bai_bien should be the coast polyline, got %+v", coast)
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

// TestNavMatchesTheGame: the server checks moves against its own copy of the nav map, so it must be the map the
// game walks on. After re-baking the nav maps in Blender, run `npm run nav:sync`.
func TestNavMatchesTheGame(t *testing.T) {
	for name, embedded := range map[string][]byte{"saigon_q1_nav.png": navPNG, "saigon_q1_nav.json": navMeta} {
		game, err := os.ReadFile(filepath.Join("..", "..", "public", "models", name))
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("no game checkout next to the server (%v)", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(game, embedded) {
			t.Errorf("server/content/nav/%s differs from public/models/%s: run `npm run nav:sync`", name, name)
		}
	}
	if _, err := Nav(); err != nil {
		t.Fatal(err)
	}
}
