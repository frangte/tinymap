// Package capability is what a character may do or use beyond the core everyone shares: an app, a uniform,
// later a vehicle. Systems check capabilities, never which archetype a character is; server/content/archetypes.json
// says which archetype grants what (docs/design/03-domain-model.md).
package capability

// Set holds capability ids such as "driver.app".
type Set map[string]struct{}

// Of is the set of the given ids.
func Of(ids ...string) Set {
	s := make(Set, len(ids))
	for _, id := range ids {
		s[id] = struct{}{}
	}
	return s
}

func (s Set) Has(id string) bool {
	_, ok := s[id]
	return ok
}

// HasAll reports whether every id is in the set (true for none).
func (s Set) HasAll(ids []string) bool {
	for _, id := range ids {
		if !s.Has(id) {
			return false
		}
	}
	return true
}
