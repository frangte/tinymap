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
			_, err := load(placesJSON, c.archetypes, c.apps)
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
