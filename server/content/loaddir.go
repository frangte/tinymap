package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDir parses and checks the data files in a directory laid out like this one (the JSON files next to it, the
// dialogue under dialogue/), with the same checks Load runs on the embedded copy. The story editor uses it to check
// a draft before it is saved.
func LoadDir(dir string) (*Catalog, error) {
	f, err := filesFromDir(dir)
	if err != nil {
		return nil, err
	}
	return load(f)
}

func filesFromDir(dir string) (files, error) {
	var f files
	for name, dst := range map[string]*[]byte{
		"places.json": &f.places, "map.json": &f.island, "archetypes.json": &f.archetypes, "apps.json": &f.apps,
		"movement.json": &f.movement, "vehicles.json": &f.vehicles, "emotes.json": &f.emotes, "npcs.json": &f.npcs,
		"contacts.json": &f.contacts, "items.json": &f.items, "quests.json": &f.quests, "ridehail.json": &f.ridehail,
		"directions.json": &f.directions, "guide.json": &f.guide, "collection.json": &f.collection, "driving.json": &f.driving,
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return f, fmt.Errorf("content: %w", err)
		}
		*dst = b
	}
	entries, err := os.ReadDir(filepath.Join(dir, "dialogue"))
	if err != nil {
		return f, fmt.Errorf("content: %w", err)
	}
	f.dialogues = map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "dialogue", e.Name()))
		if err != nil {
			return f, fmt.Errorf("content: %w", err)
		}
		f.dialogues[e.Name()] = b
	}
	return f, nil
}
