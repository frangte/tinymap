package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contentDir = "../../content"

func copyContent(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(contentDir)); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestCheckAcceptsTheShippedContent(t *testing.T) {
	if r := Check(contentDir); !r.OK || len(r.Errors) != 0 {
		t.Fatalf("shipped content refused: %v", r.Errors)
	}
}

func TestCheckRefusesBrokenContent(t *testing.T) {
	cases := map[string]struct{ file, old, new, want string }{
		"a goto to a node that is not there": {"dialogue/chu_tam.json", `"goto": "shop"`, `"goto": "no_such_node"`, "no_such_node"},
		"an unknown field":                   {"dialogue/chu_tam.json", `"start": "greet"`, `"start": "greet", "colour": 1`, "colour"},
		"a malformed quest id":               {"quests.json", `"id": "story.goi_ve_cho_ma"`, `"id": "Bad Id"`, "Bad Id"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := copyContent(t)
			path := filepath.Join(dir, c.file)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), c.old) {
				t.Fatalf("%s has no %q to break", c.file, c.old)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(b), c.old, c.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			r := Check(dir)
			if r.OK || len(r.Errors) == 0 || !strings.Contains(r.Errors[0], c.want) {
				t.Fatalf("want a refusal mentioning %q, got %+v", c.want, r)
			}
		})
	}
}

func TestCheckReportsAMissingDirectory(t *testing.T) {
	if r := Check(filepath.Join(t.TempDir(), "nope")); r.OK || len(r.Errors) == 0 {
		t.Fatalf("a missing directory passed: %+v", r)
	}
}
