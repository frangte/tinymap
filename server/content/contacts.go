package content

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
)

// Contacts (contacts.json, shared with the game, which draws the Danh bạ; docs/design/12-phone.md): the people
// a character can have the number of. A contact is an NPC of the island (its id is the NPC's: it gives its number
// to whoever has talked to it face to face) or a voice with no body in the world (Má), which has its own name,
// role and dialogue and is given to a character when it starts (`start_for`). A call is a conversation in the
// contact's dialogue, starting at the graph's `call` node. The server keeps who has which number.

//go:embed contacts.json
var contactsJSON []byte

// Contact is one number.
type Contact struct {
	// ID is stable; an NPC's contact has the NPC's id
	ID string `json:"id"`
	// Name, Role and Dialogue describe a contact with no NPC on the island; an NPC's contact has none of them
	Name     Text   `json:"name,omitempty"`
	Role     Text   `json:"role,omitempty"`
	Dialogue string `json:"dialogue,omitempty"`
	// StartFor: a character whose archetype has one of these tags has the number from the start (a contact
	// with an NPC may have it too: the number was given before they met)
	StartFor []string `json:"start_for,omitempty"`
}

func parseContacts(f files, c *Catalog) error {
	var k struct {
		About    string    `json:"about"`
		Contacts []Contact `json:"contacts"`
	}
	if err := decodeStrict(f.contacts, &k); err != nil {
		return fmt.Errorf("contacts.json: %w", err)
	}
	c.Contacts = k.Contacts
	return nil
}

// checkContacts runs after the cast and the archetypes are checked.
func (c *Catalog) checkContacts() error {
	tags := map[string]bool{}
	for _, a := range c.Archetypes {
		for _, t := range a.Tags {
			tags[t] = true
		}
	}
	seen := map[string]bool{}
	for _, k := range c.Contacts {
		if seen[k.ID] {
			return fmt.Errorf("contacts.json: contact %q: duplicate id", k.ID)
		}
		seen[k.ID] = true
		if err := c.checkContact(k, tags); err != nil {
			return fmt.Errorf("contacts.json: contact %q: %w", k.ID, err)
		}
	}
	return nil
}

func (c *Catalog) checkContact(k Contact, tags map[string]bool) error {
	if !idPattern.MatchString(k.ID) {
		return errors.New("id must be lower_snake_case ascii")
	}
	graph := k.Dialogue
	if n, ok := c.NPCByID(k.ID); ok {
		if k.Name.VI != "" || k.Role.VI != "" || k.Dialogue != "" {
			return errors.New("an NPC's contact takes its name, role and dialogue from the NPC: leave them out")
		}
		graph = n.Dialogue
	} else {
		if !bilingual(k.Name) || !bilingual(k.Role) {
			return errors.New("not an NPC id: needs a name and a role in Vietnamese and English")
		}
		if len(k.StartFor) == 0 {
			return errors.New("a contact with no NPC can only be given at the start: needs start_for")
		}
	}
	d, ok := c.Dialogues[graph]
	if !ok {
		return fmt.Errorf("dialogue %q is not in dialogue/", graph)
	}
	if d.Call == "" {
		return fmt.Errorf("dialogue/%s.json has no call node, so nobody could call them", graph)
	}
	for _, t := range k.StartFor {
		if !tags[t] {
			return fmt.Errorf("start_for tag %q is no archetype's (archetypes.json)", t)
		}
	}
	return nil
}

// Contact finds a contact by id.
func (c *Catalog) Contact(id string) (Contact, bool) {
	for _, k := range c.Contacts {
		if k.ID == id {
			return k, true
		}
	}
	return Contact{}, false
}

// ContactDialogue is the dialogue graph a call to the contact runs ("" for an unknown contact).
func (c *Catalog) ContactDialogue(id string) string {
	k, ok := c.Contact(id)
	if !ok {
		return ""
	}
	if n, ok := c.NPCByID(id); ok {
		return n.Dialogue
	}
	return k.Dialogue
}

// StartContacts are the contacts a character with these archetype tags has from the start.
func (c *Catalog) StartContacts(tags []string) []string {
	var out []string
	for _, k := range c.Contacts {
		if slices.ContainsFunc(k.StartFor, func(t string) bool { return slices.Contains(tags, t) }) {
			out = append(out, k.ID)
		}
	}
	return out
}

// VoiceContacts are the contacts with no NPC on the island (calls only).
func (c *Catalog) VoiceContacts() []Contact {
	var out []Contact
	for _, k := range c.Contacts {
		if _, ok := c.NPCByID(k.ID); !ok {
			out = append(out, k)
		}
	}
	return out
}

// CallNode is the node a call to the graph starts at ("" when it has none).
func (c *Catalog) CallNode(graph string) string { return c.Dialogues[graph].Call }
