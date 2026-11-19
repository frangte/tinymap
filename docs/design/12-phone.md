# The real phone: contacts, calls and texts

Step 2 of [10-story.md](10-story.md#14-thứ-tự-đề-xuất), after the items of [11-items.md](11-items.md): the phone gets
its basic apps. "Điện thoại, Danh bạ" calls NPCs the player has the number of, "Tin nhắn" shows the texts NPCs send,
and the newcomer's first call home to Má is the first real use. Both apps are on every phone (`phone.basic`), a
Nokla included (decisions D14 and D22 in [README.md](README.md)). Written 2026-11-19.

## What is stored

Two tables, migration `0010_phone.sql`, only adds:

- `contacts (character_id, contact_id, since)`: a number a character has. `contact_id` is a content id, so a number
  taken out of `contacts.json` stays stored and is no longer shown.
- `texts (character_id, id, sender, vi, en, sent_at, read_at)`: a text message, kept in both languages as it was
  sent (the content may change, the history does not). `id` says which message it is: `<quest>/<cycle>/<start|done>/<n>`,
  so a commit that is stored twice never sends one twice. `read_at` is null until the Tin nhắn app was opened.

A character keeps its numbers and texts across archetype changes.

## Content

`server/content/contacts.json` (shared with the game, which draws the Danh bạ):

```json
{ "contacts": [
  { "id": "ma", "name": {"vi": "Má", "en": "Mum"}, "role": {"vi": "má ở quê", "en": "your mum back home"},
    "dialogue": "ma", "start_for": ["newcomer"] },
  { "id": "chu_sau" }, { "id": "anh_bay" }
] }
```

- A contact whose `id` is an NPC's (`npcs.json`) is that NPC: name and role come from the NPC, a call runs the NPC's own
  dialogue, and the number is given by **talking to them face to face** (the `npc.talked` event). Six NPCs give one:
  Anh Bảy, Chú Sáu, Chú Tư, Cô Ba, Dì Năm, Chị Hạnh.
- A contact with no NPC on the island (Má) has its own `name`, `role` and `dialogue`, and `start_for`: the archetype
  tags that have the number from the start. The newcomer has Má's number when it arrives (and when it changes to the
  newcomer, since `Arrive` runs again on Đổi vai); nobody else does. Chosen over a field in `archetypes.json`, which
  the story agent shares, and because it keeps every number in one file.
- A **text** from someone also saves their number: whoever texts you has your number and you have theirs. So a native
  who is texted by Chú Tư can call him, even before meeting.
- Every contact's dialogue graph has a **`call` node** (`"call": "<node>"` next to `start`): where a call starts.
  Content validation (`content/contacts.go`, `cast.go`) fails when a contact's graph has none, when the node does not
  exist, and counts it as a start for the "every node can be reached" check. A graph without a call node can have no
  contact, so nobody can call someone who would not answer.
- `quests.json`: a quest may have `texts: { "on_start": [...], "on_done": [...] }`, each `{ "from": <contact id>,
  "text": {vi, en} }` (at most 3 a moment, 160 characters; checked: the sender is a contact). A once-only quest may pay
  up to 200.000 xu (a daily or an achievement still 20.000): Má's 100.000đ.
- `apps.json`: `calls` (Điện thoại) and `messages` (Tin nhắn), `requires: ["phone.basic"]`, ahead of `notices`, so a
  Nokla shows Điện thoại, Tin nhắn, Thông báo.

## Calls

A call is a conversation, run by the same dialogue engine and the same graphs.

- The Danh bạ's **Gọi** sends `{"t":"call","contact":"ma","rid":n}`. The world (`internal/world/call.go`) answers
  with a `result`: `phone` (no `phone.basic`), `target` (no such contact, a guest, or a number the character does not
  have: asked of the store in a job), `mute` (no dialogue), `busy` (the store failed). Distance does not matter.
- On success the conversation starts at the graph's **call node**, as an `interaction` carrying `call: "<contact id>"`,
  and goes on with `choose` / `interact_leave` like any other. It is a `session` with `call: true`: private whatever the
  NPC's mode; it never ends by walking away; the NPC does not turn, gesture or count it as busy (`talks`); and no
  `npc.talked` event is published (a call does not meet someone, and a talk quest is not done by phone).
- **One conversation at a time**, as before: a call ends the conversation the player is in (`interaction_end` `other`),
  and walking up to an NPC ends a call. The same contact again shows the line again.
- **Signals** sent by the answers reach quests exactly like face to face ones (the event names the contact's id as
  `npc`), so a call to Chú Tư can do a `dialogue_signal` objective if its graph says so.
- A contact with no body (Má) is a **voice**: the world keeps a hidden NPC for it (`Config.Voices`), never listed,
  placed or seen, and `interact` on its id is refused.
- The game's dialogue sheet shows a call as a call: dashed border and a "Cuộc gọi" / "Phone call" tag instead of the
  group tag; the name and role come from the contact.

## Texts

- A quest's start or completion writes its texts **in the same transaction as the quest's progress**
  (`store.SaveQuests`), so they are neither lost nor sent twice when a save is retried; the sender's number is kept
  with them.
- A text reaches the game as `{"t":"text","text":{id,from,say,at,read}}` when the character has a phone (the quest
  service knows its capabilities), and the game shows a notification and a badge on Tin nhắn. Without a phone it waits
  in the store; when the character gets its first phone (`Service.Capabilities` sees `phone.basic` appear) the phone
  service sends the whole history. On joining, `{"t":"phone","contacts":[...],"texts":[...]}` carries the numbers
  and, with a phone, the texts; the game replaces what it had.
- Opening Tin nhắn sends `{"t":"texts_read"}`; the server marks every text read.
- The Crab notices that exist today (the Thông báo app, made by the game from the ride messages) are untouched.

## Pieces

| Where | What |
|---|---|
| `server/internal/phone` | the service: numbers by talking (event), starting numbers and history on arriving, history when a phone comes, read marks; store calls retried |
| `server/internal/store/phone.go` | `AddContacts`, `Contacts`, `HasContact`, `Texts`, `ReadTexts`; texts saved inside `SaveQuests` |
| `server/internal/quest` | `texts` on a quest become `Outcome.Texts` / `Commit.Texts`; the service sends them and tells `OnPhone` |
| `server/internal/world/call.go` | `call` messages and the call sessions |
| `src/game/contacts.ts`, `src/game/phone/callApps.ts` | the book of numbers and texts, the two apps |

## The call home (the newcomer, `tags_any: ["newcomer"]`)

- `story.goi_ve_cho_ma` "Gọi về cho má": `requires_caps: ["phone.basic"]` only, **not** the phone quest's completion
  (a newcomer who got a phone any other way still needs it). Its objective is the signal `ma_gui_tien`, sent by the
  answer "Dạ, con tới rồi má. Con mới mua được cái điện thoại." in Má's call. Reward: 100.000 xu. On done, Má texts.
- `story.gap_chu_sau` "Đi gặp chú Sáu": after the call; objective `talk_npc` Chú Sáu. His greeting has a `newcomer`
  variant that knows the player is his old friend's child; his other lines are unchanged.
- `citizen.ca_phe_sang` (natives): Chú Tư texts on start ("Sáng mai con ghé quán sớm nha…") and on done.

## Not built

Hints on a stuck quest by phone, a call that sends the player a quest, calls to a taxi driver (Chú Sáu to pick you up),
group texts, the Crab hotline texting, replying to a text, sound.
