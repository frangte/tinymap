package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirReadsTheSameCatalogAsLoad(t *testing.T) {
	embedded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	disk, err := LoadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Quests) != len(embedded.Quests) || len(disk.Dialogues) != len(embedded.Dialogues) || len(disk.NPCs) != len(embedded.NPCs) {
		t.Fatalf("disk has %d quests, %d dialogues, %d NPCs; embedded %d, %d, %d", len(disk.Quests), len(disk.Dialogues), len(disk.NPCs), len(embedded.Quests), len(embedded.Dialogues), len(embedded.NPCs))
	}
}

func TestLoadDirRefusesWhatLoadRefuses(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(".")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "quests.json"), []byte(`{"quests": [{"id": "x", "surprise": true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("an unknown field in quests.json was accepted")
	}
	if _, err := LoadDir(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing directory was accepted")
	}
}
