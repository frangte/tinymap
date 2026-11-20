// Command contentcheck runs the checks the server runs on its content at startup (content.Load, then the quest
// engine's compile) on a directory of content files, and prints {"ok":bool,"errors":[...]} as JSON. The story editor
// (tools/story) runs it on a draft before a save. Exit status is 0 whenever the check ran, whatever it found.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/quest"
)

// Result is what the command prints.
type Result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
}

// Check loads the directory the way the server does and lists what it refuses.
func Check(dir string) Result {
	catalog, err := content.LoadDir(dir)
	if err != nil {
		return Result{Errors: []string{err.Error()}}
	}
	if _, err := quest.NewEngine(catalog); err != nil {
		return Result{Errors: []string{err.Error()}}
	}
	return Result{OK: true, Errors: []string{}}
}

func main() {
	dir := flag.String("dir", "content", "directory with the content files (server/content)")
	flag.Parse()
	if err := json.NewEncoder(os.Stdout).Encode(Check(*dir)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
