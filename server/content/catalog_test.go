package content

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

func TestCatalogLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Archetype(""); ok {
		t.Fatal(`"" is no archetype`)
	}
	for _, a := range c.Archetypes {
		caps := c.Grants(a.ID)
		apps := c.AppsFor(caps)
		// everyone has the core apps; each archetype adds at least one of its own
		for _, core := range []string{"profile", "chat", "map", "notices"} {
			if !slices.Contains(apps, core) {
				t.Errorf("%s has no %s app", a.ID, core)
			}
		}
		if len(apps) <= 4 {
			t.Errorf("%s gets no app of its own: %v", a.ID, apps)
		}
	}
	// no archetype: the core apps only, and no uniform
	if apps := c.AppsFor(c.Grants("")); len(apps) != 4 {
		t.Errorf("apps without an archetype = %v", apps)
	}
}

func TestRestrictedLooks(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	crab := protocol.Look{Body: "woman", Outfit: "crab", Hat: "helmet"}
	none := capability.Of()
	uniform := capability.Of("uniform.crab")
	if c.LookAllowed(crab, none) {
		t.Fatal("the Crab Bike uniform needs uniform.crab")
	}
	if !c.LookAllowed(crab, uniform) {
		t.Fatal("uniform.crab may wear it")
	}
	if got := c.AllowedLook(crab, none); got != (protocol.Look{Body: "woman", Outfit: fallbackOutfit, Hat: fallbackHat}) {
		t.Fatalf("AllowedLook = %+v", got)
	}
	open := protocol.Look{Body: "man", Outfit: "aodai", Hat: "nonla"}
	if c.AllowedLook(open, none) != open || !c.LookAllowed(open, none) {
		t.Fatal("open looks are left alone")
	}
}

func TestCatalogRejects(t *testing.T) {
	const caps = `"capabilities": [{"id": "a.app", "about": "x"}, {"id": "a.uniform", "about": "x"}]`
	archetype := func(fields string) string {
		base := map[string]string{
			"id": `"a"`, "name": `{"vi": "A"}`, "pitch": `{"vi": "A"}`,
			"badge":        `{"short": "A", "color": "#112233", "text": "#FFFFFF"}`,
			"spawn":        `{"place": "cho_ben_thanh", "at": [0, 0], "face": [1, 1], "button": {"vi": "Về"}}`,
			"look":         `{"outfit": "thun", "hat": "none"}`,
			"capabilities": `["a.app"]`,
		}
		for _, kv := range strings.Split(fields, ";") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				base[k] = v
			}
		}
		parts := []string{}
		for k, v := range base {
			parts = append(parts, `"`+k+`": `+v)
		}
		return `{` + strings.Join(parts, ", ") + `}`
	}
	file := func(archetypes, restricted string) []byte {
		return []byte(`{` + caps + `, "archetypes": [` + archetypes + `], "restricted_looks": [` + restricted + `]}`)
	}
	okApps := []byte(`{"apps": [{"id": "profile", "name": {"vi": "Hồ sơ"}, "icon": "p"}]}`)
	cases := []struct {
		name       string
		archetypes []byte
		apps       []byte
		want       string
	}{
		{"unknown spawn place", file(archetype(`spawn={"place": "nowhere", "at": [0, 0], "face": [1, 1], "button": {"vi": "Về"}}`), ""), okApps, "spawn place"},
		{"unknown capability", file(archetype(`capabilities=["a.nope"]`), ""), okApps, "unknown capability"},
		{"bad colour", file(archetype(`badge={"short": "A", "color": "blue", "text": "#FFFFFF"}`), ""), okApps, "badge"},
		{"unknown outfit", file(archetype(`look={"outfit": "suit", "hat": "none"}`), ""), okApps, "starter"},
		{"starter look it may not wear", file(archetype(`look={"outfit": "crab", "hat": "none"}`), `{"outfit": "crab", "requires": "a.uniform"}`), okApps, "starter look"},
		{"restricting both parts at once", file(archetype(""), `{"outfit": "crab", "hat": "helmet", "requires": "a.uniform"}`), okApps, "exactly one"},
		{"restricting the fallback", file(archetype(""), `{"outfit": "thun", "requires": "a.uniform"}`), okApps, "starter look"},
		{"app needs an unknown capability", file(archetype(""), ""), []byte(`{"apps": [{"id": "x", "name": {"vi": "X"}, "icon": "x", "requires": ["a.nope"]}]}`), "unknown capability"},
		{"unknown field", []byte(`{` + caps + `, "archetypes": [` + archetype(`colour="red"`) + `]}`), okApps, "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			// the shipped vehicles need capabilities these small files do not define
			f.archetypes, f.apps, f.vehicles = c.archetypes, c.apps, []byte(`{"vehicles": []}`)
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestMotionContent(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Movement.RunSpeed <= c.Movement.WalkSpeed || c.Movement.Tolerance < 1 {
		t.Fatalf("movement = %+v", c.Movement)
	}
	var riders []string
	for _, a := range c.Archetypes {
		if v, ok := c.StarterVehicle(a.ID, c.Grants(a.ID)); ok {
			if v.Speed <= c.Movement.RunSpeed {
				t.Errorf("%s's vehicle is no faster than running", a.ID)
			}
			riders = append(riders, a.ID)
		}
	}
	if len(riders) == 0 {
		t.Fatal("no archetype starts with a vehicle: the driver should")
	}
	if !c.EmoteAllowed(0, c.Grants("")) || c.EmoteAllowed(MaxEmoteCode+1, c.Grants("")) {
		t.Fatal("code 0 is no emote and always allowed; unknown codes are not")
	}
	for _, e := range c.Emotes {
		if !c.EmoteAllowed(e.Code, c.Grants("")) && e.Requires == "" {
			t.Errorf("emote %s needs nothing but is refused", e.ID)
		}
	}
}

func TestMotionRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(*files)
		want string
	}{
		{"running slower than walking", func(f *files) {
			f.movement = []byte(`{"walk_speed": 7, "run_speed": 3, "jump_speed": 4, "gravity": 15, "tolerance": 1.25}`)
		}, "movement.json"},
		{"a vehicle at 110 m/s", func(f *files) {
			f.vehicles = []byte(`{"vehicles": [{"id": "x", "name": {"vi": "X"}, "speed": 110, "seats": 1, "requires": "vehicle.motorbike.ride"}]}`)
		}, "speed"},
		{"a vehicle needing an unknown capability", func(f *files) {
			f.vehicles = []byte(`{"vehicles": [{"id": "xe_so_110", "name": {"vi": "X"}, "speed": 11, "seats": 1, "requires": "nope.nope"}]}`)
		}, "unknown capability"},
		{"two emotes on one code", func(f *files) {
			f.emotes = []byte(`{"emotes": [{"id": "a", "code": 1, "name": {"vi": "A"}, "seconds": 1}, {"id": "b", "code": 1, "name": {"vi": "B"}, "seconds": 1}]}`)
		}, "code"},
		{"an emote that lasts an hour", func(f *files) {
			f.emotes = []byte(`{"emotes": [{"id": "a", "code": 1, "name": {"vi": "A"}, "seconds": 3600}]}`)
		}, "seconds"},
		{"an emote code past four bits", func(f *files) {
			f.emotes = []byte(`{"emotes": [{"id": "a", "code": 16, "name": {"vi": "A"}, "seconds": 1}]}`)
		}, "code"},
		{"a starter vehicle that does not exist", func(f *files) {
			f.vehicles = []byte(`{"vehicles": []}`)
		}, "starter vehicle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			c.edit(&f)
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

// TestNoArchetypeIDsInCode keeps archetypes data: code checks capabilities, never which archetype a character is
// (docs/design/03-domain-model.md). Only the data files and tests may name an archetype.
func TestNoArchetypeIDsInCode(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(c.Archetypes))
	for _, a := range c.Archetypes {
		ids = append(ids, regexp.QuoteMeta(a.ID))
	}
	quoted := regexp.MustCompile("[\"'`](" + strings.Join(ids, "|") + ")[\"'`]")
	for _, root := range []string{"..", filepath.Join("..", "..", "src")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "data") {
				return filepath.SkipDir
			}
			ext := filepath.Ext(path)
			if d.IsDir() || (ext != ".go" && ext != ".ts") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if m := quoted.Find(b); m != nil {
				t.Errorf("%s names archetype %s: check a capability instead (server/content/archetypes.json)", path, m)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
