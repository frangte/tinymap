# Story flags and quest outcomes

Mechanisms 1 and 2 of [14-story-tools.md](14-story-tools.md) (D26). Written 2026-11-20. Khoa approved the direction, not
a story: nothing in the shipped content uses these yet, and the tests use a fixture
(`server/internal/quest/testdata/branching_story.json`). This file is for whoever writes the first branch.

## What it adds

- A **flag** is a fact about a character ("gave the car back"), kept in the database, never cleared in v1.
- A quest can **end in one of several outcomes**. Which one is recorded, sets flags, and gives its own reward.
- Quests, dialogue answers and dialogue lines can **read flags**.

The architecture rule stays: the dialogue provider (`server/internal/dialogue`) only moves along edges, sends signals and
reads flags. It never sets one. Flags come from the quest engine, which sets them in the same transaction that
completes the quest and stores its rewards.

## Content format

### Outcomes (`quests.json`, on a quest)

```json
"outcomes": [
  {
    "id": "tra_xe",
    "type": "dialogue_signal",
    "params": { "npc": "chu_tam", "signal": "tra_xe" },
    "text":  { "vi": "Trả xe cho Chú Tám", "en": "Give the bike back" },
    "at": "chu_tam",
    "title": { "vi": "Trả xe", "en": "Gave it back" },
    "flags": ["da_tra_xe"],
    "rewards": { "xu": 20000, "stamps": ["..."] },
    "texts": { "on_done": [{ "from": "ma", "text": { "vi": "...", "en": "..." } }] }
  }
]
```

An outcome is an **objective** (`id`, `type`, `params`, `count`, `text`, `at`: the same types and param checks as
`objectives`) plus:

| Field | Meaning |
|---|---|
| `title` | `{vi, en}`, shown in the quest log once the quest is done ("ending: ...") |
| `flags` | story flags set when the quest ends with this outcome |
| `rewards` | titles, stamps, capability, xu, given besides the quest's own. No `rewards.flags` here (use `flags`) |
| `texts.on_done` | phone texts sent besides the quest's own `texts.on_done` |

Rules (checked by `server/content/quests.go`, `flags.go`):

- 2 to 6 outcomes. One outcome would just be an objective.
- Not on dailies or jobs, and not on achievements.
- An outcome id is `lower_snake_case` and unique among the quest's objective ids **and** outcome ids (their progress is stored
  in one map).
- `objectives` may be empty: the outcomes are live from the start (a pure choice).
- Reward caps: the quest's xu plus an outcome's xu stay within `XuStoryRewardMax` (200.000đ), for every outcome.

### Flags

- `rewards.flags` (quest): flags set when the quest completes, with or without outcomes. Quests only (not dailies, jobs or
  achievements).
- Ending with outcome `X` also sets the flag `<quest id>:<X>` by itself (`test.fork:hand_over`), plus the outcome's `flags` and
  the quest's `rewards.flags`.
- Flag id: `^[a-z0-9]+(_[a-z0-9]+)*(\.[a-z0-9]+(_[a-z0-9]+)*)*$`, or the outcome form `<quest id>:<outcome id>` (read-only: it
  cannot be listed in `flags`).
- Flags are **not declared** anywhere. Validation requires every flag that is read to be set by some quest. A quest cannot
  read a flag only it sets.

### Readers

| Where | Field | Meaning |
|---|---|---|
| Quest | `requires_flags` (all), `unless_flags` (any) | when the quest is offered. A running quest keeps running. Availability updates live as flags change |
| Dialogue answer | `if_flags` (all), `unless_flags` (any) | whether the answer is offered |
| Dialogue line | `say` key `"flag:<flag id>"` | line for characters holding the flag |

Line choice order in `variant` (`dialogue.go`): a `flag:` key first (alphabetical among the flags held), then the archetype,
then a capability key, then `default`. `requires_flags` and `unless_flags` (and `if_flags`/`unless_flags`) may not list the
same flag.

## Semantics

1. The quest's `objectives` run per `flow` as before.
2. When they are all done (at once if there are none), the **outcomes go live together**. An answer counted for an outcome
   before that does nothing. The event that completes the last objective does not also count for an outcome.
3. The first outcome whose objective completes ends the quest. If one event completes two, the first in file order wins.
   Later events for the other outcomes change nothing.
4. The player gets the quest's rewards and `on_done` texts, then the outcome's. The grants are two, with two sources
   (`quest:<id>:` and `quest:<id>:outcome:<outcome id>`), so each is given once.
5. Flags are set: `<quest id>:<outcome id>`, the outcome's `flags`, the quest's `rewards.flags`.
6. The `QuestDone` event follows, so achievements and `quest_done` objectives work as for any quest. A quest that requires
   the quest with `requires` opens whatever outcome it ended with; use `requires_flags` for a branch.

Signals of one conversation apply in order: the quest service runs each character on one worker with a FIFO queue, so an
answer that completes the last objective and the answer that picks an outcome right after both count
(`TestSignalsOfOneConversationApplyInOrderAndSetFlagsLive`).

## Storage (`0013_story_flags.sql`)

- `story_flags (character_id, flag, set_at)`, primary key `(character_id, flag)`.
- `quest_progress.outcome text not null default ''`: the outcome a done quest ended with.
- `SaveQuests` inserts flags in the same transaction as the record, grants and texts, with `on conflict do nothing`. A retry
  is a no-op and a flag keeps the time it was first set.
- Outcome progress lives in `quest_progress.objectives` under the outcome id.

## World and dialogue

`dialogue.Speaker.Flags` is the character's flags. The world keeps them per member (`member.story`) like `quests` and `caps`.
The quest service pushes them with `world.SetFlags` whenever it sends progress (on join, and after any step), so they are
loaded on join. An answer that sets a flag is rendered before the quest service has heard of it, so when the flags
change during a conversation the world shows the current line again (the leader's flags re-render for the whole group), and
the next line already reflects them.

## Protocol (`progress`, `QuestState`)

A done quest carries `outcome`; a running quest whose objectives are done carries `outcomes`, the live ones in quest order:

```json
{ "id": "chuong_4.chiec_xe", "n": {"tim_xe": 1}, "outcomes": [ {"id": "tra_xe", "n": 0}, {"id": "giu_xe", "n": 0} ] }
{ "id": "chuong_4.chiec_xe", "done": true, "outcome": "tra_xe" }
```

The client has the text from `quests.json`. The tracker shows the live outcomes as alternatives joined by "hoặc" / "or"
(`src/game/questTracker.ts`); the Sổ tay quest tab shows them under "Chọn một cách kết thúc:" and the done list shows
"kết cục: <title>" (`sections.ts`). Flags are not sent to the client.

## How to author a branch

1. Write the quest with its `objectives` and 2 to 6 `outcomes`. Each outcome is usually a `dialogue_signal` on one of the NPC's
   answers, so the NPC's dialogue needs those answers (the `Emits` check fails otherwise).
2. Gate or colour dialogue with `if_flags`, `unless_flags` and `flag:` say keys. Chapter 5 can read
   `chuong_4.chiec_xe:tra_xe` directly.
3. Open the next quests with `requires_flags: ["chuong_4.chiec_xe:tra_xe"]`.
4. Run `cd server && go test ./...`: content validation catches a flag nobody sets, a misspelt outcome, a `flag:` line for nothing.
5. Check both languages (`?lang=vi`, `?lang=en`).

## Limits (v1)

- Flags are never cleared and have no values (no counters: `than_chu_sau: +1` of 14-story-tools.md is not built).
- Not on dailies, jobs or achievements.
- No requirement cycles check across `requires_flags` beyond "not only set by itself": two quests that wait for each other's
  flags would never open. Keep branches one way.
- The `outcomes` state sent has no timers (no `left`), so use outcomes whose objectives are not errands.
- Content stays untested end to end for the fixture only: no shipped quest uses outcomes yet.
- Not yet: delayed events (3), clues (4), per-character variants (5), hidden quests (6), and the workshop (part 2).
