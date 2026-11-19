# Items, the phone shop and the new start

Step 2 of [10-story.md](10-story.md#14-thứ-tự-đề-xuất): a character picks an origin, starts with money and no phone,
and buys one in a shop. Written 2026-11-19 as the plan the step is built from (decision D22). Khoa's answers that
day: the origin can still be changed in game (Đổi vai), money shows as đồng, nobody starts with a phone and the game
says buying one is the first quest, the names Nokla and Ifone are fine.

## Capabilities

Two new capabilities in `server/content/archetypes.json`, given by items, never by an archetype:

| Capability | Given by | Opens |
|---|---|---|
| `phone.basic` | every phone | the phone itself: notifications, later calls, contacts and messages |
| `phone.smart` | a smartphone (with `phone.basic`) | every other app: Crab, chat, Hỏi đường, Hàng Xóm, Crab Driver |

`apps.json` says so with `requires`, as for any hidden app. A character without `phone.basic` has no apps, and the
game shows no phone: it says where to buy one. Nearby chat (the chat box) never needs a phone. Booking a Crab
needs `phone.smart` on the server too (a tampered game cannot book without one).

## Items (`server/content/items.json`)

```json
{
  "about": "…",
  "shops": [
    { "id": "de_yeu", "npc": "anh_bay", "name": { "vi": "Tiệm điện thoại Dế Yêu", "en": "Dế Yêu phone shop" } }
  ],
  "items": [
    { "id": "nokla_1280", "kind": "phone", "shop": "de_yeu", "price": 150000, "grants": ["phone.basic"],
      "name": { "vi": "Nokla 1280", "en": "Nokla 1280" }, "about": { "vi": "…", "en": "…" } }
  ]
}
```

- Phones, in this order: Nokla 1280 (150.000đ, `phone.basic`), Ifone X (400.000đ), Ifone 11 (1.200.000đ), Ifone 15
  Pro (6.000.000đ); the three Ifones give `phone.basic` and `phone.smart`. Prices are game prices (10-story.md,
  section 11).
- An item is bought once. A character may own several phones; the one in hand is the last owned in file order.
- Ids are stable: characters store them. Every `grants` capability is declared in archetypes.json, every `shop`
  exists, every shop's `npc` is in npcs.json. Text is `{vi, en}` like all content.

## Storage

Migration `0009_items.sql`: `owned_items (character_id, item_id, price, bought_at)`, primary key
`(character_id, item_id)`. A purchase is one transaction: the item row (`on conflict do nothing`: a retried
purchase is a no-op), the balance down by the price only if it covers it (else nothing is kept), a ledger row
(reason `buy`), and a `capability_grants` row per capability with source `item:<id>`. Every character from before
items gets an Ifone X for free (10-story.md, section 13: "ai cũng được một smartphone").

## Messages

- Game → server: `{"t":"buy","rid":7,"item":"ifone_x"}`.
- Server → game, on success: `wallet` (the new balance), then `{"t":"items","items":["ifone_x"],"apps":[…]}` (every
  item owned, and the phone apps the character has now), then `result` ok. On failure only `result` with a code:
  `far` (not next to the shop's NPC), `target` (no such item), `owned` (already has it), `balance` (cannot pay).
- `welcome` gets `items` (every item owned; absent for a guest without an account).
- A talk `interaction` line gets `shop` (a shop id) when its dialogue node has `"shop": "de_yeu"`: the game shows a
  button that opens the shop sheet. The sheet lists the shop's items with price and what each opens, and buys with
  `buy`.
- A purchase sends the quest event `item.bought` (Item, ItemKind). The objective `buy_item` takes
  `{"item": "<id>"}` or `{"kind": "phone"}`.
- A quest may have `unless_caps`: it is not offered to a character that has any of them (the phone quest is not
  offered to someone who already has a phone).

## Origins (`archetypes.json`)

| id | Name | Tags | Starts at | Clothes |
|---|---|---|---|---|
| `newcomer` (new) | Người từ quê lên / From the provinces | `citizen`, `newcomer` | Bến xe Chợ Lớn | plain |
| `citizen` | Người Sài Gòn gốc / Native Saigonese | `citizen` | dãy nhà phố Lê Lai | plain |
| `traveler` | Người nước ngoài / Foreign backpacker | `traveler` | phố đi bộ Bùi Viện | plain |
| `driver` | Tài xế Crab Bike | `driver` | as now | as now |

`driver` gets `"job": true`: the first pick shows only the three origins, Đổi vai shows the driver too until the
job is earned in chapter 3 (step 4), when the driver archetype goes. Everyone starts with 500.000đ
(`ridehail.json` `start_balance`); a foreigner's 20 USD and Cô Kim's exchange come later.

Lines may use `{anh_chi}`: "anh" for a man's body, "chị" for a woman's, in Vietnamese only (English lines never
carry it). Older NPCs say "con" and need no variable.

## Money on screen

Balances and prices show as đồng: `500.000đ` in Vietnamese, `500,000₫` in English. The code, content and the
protocol keep whole xu (1 xu = 1 đồng).

## Order

1. In parallel: the server (items, store, messages, the shop module, `buy_item`, `unless_caps`, the phone quest and
   Anh Bảy with a first dialogue), the game (no phone without one, the shop sheet, the phone and items in the Sổ tay,
   đồng, the first-quest card), the origins (`newcomer`, the picker, `{anh_chi}`).
2. Then the content of chapter 0 (Bà Chín, Anh Tony, the openings of each origin) and the real phone (contacts,
   calls, messages).
3. Then Cô Kim's exchange and the first Crab ride for each origin.
