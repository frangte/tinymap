package content

import (
	"slices"
	"strings"
	"testing"
)

func TestShippedItems(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	shop, ok := c.Shop("de_yeu")
	if !ok {
		t.Fatal("the Dế Yêu shop is missing")
	}
	if n, ok := c.NPCByID(shop.NPC); !ok || n.ID != "anh_bay" {
		t.Fatalf("the shop's npc = %+v, %v", n, ok)
	}
	var ids []string
	for _, it := range c.ShopItems("de_yeu") {
		ids = append(ids, it.ID)
	}
	// the order is the order the sheet lists them in, and the phone in hand is the last owned
	if want := []string{"nokla_1280", "ifone_x", "ifone_11", "ifone_15_pro"}; !slices.Equal(ids, want) {
		t.Fatalf("items = %v, want %v", ids, want)
	}
	nokla, _ := c.Item("nokla_1280")
	if nokla.Price != 150_000 || !slices.Equal(nokla.Grants, []string{CapPhone}) {
		t.Fatalf("nokla = %+v", nokla)
	}
	for _, id := range ids[1:] {
		it, _ := c.Item(id)
		if !slices.Equal(it.Grants, []string{CapPhone, CapSmartphone}) {
			t.Errorf("%s grants %v: an Ifone is a smartphone", id, it.Grants)
		}
	}
	if _, ok := c.Item("nope"); ok {
		t.Fatal("an unknown item was found")
	}
	if len(c.ShopItems("nope")) != 0 {
		t.Fatal("an unknown shop sells nothing")
	}
}

// Anh Bảy's shop node opens his shop, and the first greeting reaches it.
func TestAnhBaySellsFromTheShopNode(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	d, ok := c.Dialogue("anh_bay")
	if !ok {
		t.Fatal("anh_bay has no dialogue")
	}
	shopNodes := 0
	for _, n := range d.Nodes {
		if n.Shop == "de_yeu" {
			shopNodes++
		}
	}
	if shopNodes != 1 {
		t.Fatalf("%d nodes open the shop, want 1", shopNodes)
	}
	reached := slices.ContainsFunc(d.Nodes[d.Start].Choices, func(ch Choice) bool { return d.Nodes[ch.Goto].Shop == "de_yeu" })
	if !reached {
		t.Fatal("the greeting does not lead to the shop")
	}
}

func TestItemsReject(t *testing.T) {
	const shop = `{"id": "de_yeu", "npc": "chu_tu", "name": {"vi": "Tiệm", "en": "Shop"}}`
	item := func(fields string) string {
		base := map[string]string{
			"id": `"a"`, "kind": `"phone"`, "shop": `"de_yeu"`, "price": `100`, "grants": `["phone.basic"]`,
			"name": `{"vi": "A", "en": "A"}`, "about": `{"vi": "A", "en": "A"}`,
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
	// the shops and fees the shipped dialogues open must exist whatever else a case replaces
	const others = `{"id": "xe_chu_tam", "npc": "chu_tam", "name": {"vi": "X", "en": "X"}}, {"id": "van_phong_crab", "npc": "chi_vy", "name": {"vi": "V", "en": "V"}}`
	const fees = `{"id": "thi_a1", "npc": "thay_muoi", "amount": 200000, "grants": ["exam.a1.ticket"], "name": {"vi": "T", "en": "T"}, "about": {"vi": "T", "en": "T"}},
		{"id": "thi_lai_a1", "npc": "thay_muoi", "amount": 50000, "grants": ["exam.a1.ticket"], "name": {"vi": "T", "en": "T"}, "about": {"vi": "T", "en": "T"}}`
	file := func(shops, items string) []byte {
		sep := ""
		if shops != "" {
			sep = ", "
		}
		return []byte(`{"shops": [` + shops + sep + others + `], "fees": [` + fees + `], "items": [` + items + `]}`)
	}
	cases := []struct {
		name string
		file []byte
		want string
	}{
		{"an id with capitals", file(shop, item(`id="Nokla"`)), "lower_snake_case"},
		{"two items with one id", file(shop, item("")+", "+item("")), "duplicate"},
		{"an unknown kind", file(shop, item(`kind="car"`)), "unknown kind"},
		{"a shop that does not exist", file(shop, item(`shop="nope"`)), "not in items.json"},
		{"a free item", file(shop, item(`price=0`)), "price"},
		{"a negative price", file(shop, item(`price=-5`)), "price"},
		{"a price nobody could pay", file(shop, item(`price=1000000000`)), "price"},
		{"a capability nobody declared", file(shop, item(`grants=["no.such"]`)), "unknown capability"},
		{"a capability twice", file(shop, item(`grants=["phone.basic","phone.basic"]`)), "twice"},
		{"no English name", file(shop, item(`name={"vi": "A"}`)), "English"},
		{"no English about", file(shop, item(`about={"vi": "A"}`)), "English"},
		{"a shop whose npc does not exist", file(`{"id": "de_yeu", "npc": "nobody", "name": {"vi": "T", "en": "S"}}`, ""), "npcs.json"},
		{"two shops with one id", file(shop+", "+shop, ""), "duplicate"},
		{"a shop without an English name", file(`{"id": "de_yeu", "npc": "chu_tu", "name": {"vi": "T"}}`, ""), "English"},
		{"an unknown field", []byte(`{"shops": [], "items": [], "extra": 1}`), "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.items = c.file
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
	// the base case loads, so each failure above is the one edit
	f := embedded()
	f.items = file(shop, item(""))
	if _, err := load(f); err != nil {
		t.Fatalf("the base case: %v", err)
	}
}

func TestADialogueNodeCannotOpenAnUnknownShop(t *testing.T) {
	f := embedded()
	shipped := string(f.dialogues["anh_bay.json"])
	f.dialogues = map[string][]byte{}
	for name, b := range embedded().dialogues {
		f.dialogues[name] = b
	}
	f.dialogues["anh_bay.json"] = []byte(strings.Replace(shipped, `"shop": "de_yeu"`, `"shop": "nope"`, 1))
	if _, err := load(f); err == nil || !strings.Contains(err.Error(), `shop "nope"`) {
		t.Fatalf("err = %v", err)
	}
}
