package content

import (
	_ "embed"

	"saigon-noir/server/internal/nav"
)

// The nav map the game walks on (public/models/saigon_mini_nav.*), copied here so the server binary carries it:
// Go can only embed files inside its module. TestNavMatchesTheGame fails when the copies drift; after re-baking
// the nav maps in Blender, run `npm run nav:sync`.
var (
	//go:embed nav/saigon_mini_nav.png
	navPNG []byte
	//go:embed nav/saigon_mini_nav.json
	navMeta []byte
)

// Nav loads the embedded nav grid.
func Nav() (*nav.Grid, error) { return nav.Load(navPNG, navMeta) }
