# System architecture

One Go process, organized as modules inside a single program. The world loop owns everything real-time; gameplay services own everything durable.

## Module map

```mermaid
flowchart TB
  subgraph C["Browser: Three.js + TypeScript"]
    direction LR
    REN["Scene: avatars, NPCs, vehicles"]
    INP["Input: keys, joystick, buttons"]
    UI["DOM UI: phone, dialogue, quest tracker, chat"]
    NET["net: socket router + REST client"]
  end

  subgraph S["Go server: one process, modular monolith"]
    GW["transport: ws + http"]
    subgraph RT["World loop (one goroutine per city copy)"]
      WRL["entities, interest grid, movement check, mounts and seats"]
      NPCB["NPC behaviour"]
      INT["interaction sessions"]
      CHN["nearby chat routing"]
    end
    BUS(("in-process event bus"))
    subgraph GP["Gameplay services (do the DB work, off the world loop)"]
      IDN["identity + character"]
      ARC["archetype + capability"]
      QST["quest engine"]
      ACH["achievements"]
      INV["inventory"]
      DLG["dialogue engine"]
      CHT["chat + moderation"]
      PHN["phone app registry"]
      NTF["notifications"]
      RID["ride-hailing (future, leaf module)"]
    end
  end

  DB[("SQLite")]
  CNT[/"content/*.yaml: archetypes, quests, npcs, dialogue, items, places, apps"/]

  NET <-->|"WebSocket"| GW
  NET -->|"REST"| GW
  GW --> RT
  GW --> GP
  RT -->|"PlaceEntered, NpcInteracted, EmoteDone, Mounted"| BUS
  DLG -->|"DialogueSignal"| BUS
  INV -->|"ItemUsed, ItemGained"| BUS
  BUS --> QST
  BUS --> ACH
  QST --> INV
  QST --> NTF
  ACH --> NTF
  NTF --> GW
  GP --> DB
  CNT --> ARC & QST & DLG & INV & PHN
```

## How the modules talk

- **The world loop never touches the database.** It publishes facts to the in-process event bus, e.g. "Lan entered Nhà thờ Đức Bà".
- **Gameplay services handle those events in per-character workers.** Each character always maps to the same worker goroutine. One character's updates happen in order, with no lost writes, while different characters run in parallel.
- **Messages back to a player go through the world loop.** That keeps today's rule that one goroutine owns the connection queues (`server/internal/hub/hub.go:2`).
- **Nothing in the core depends on archetype modules.** `ridehail` may import world, inventory and notify; none of them import it.
- **Systems check capabilities and tags, never the archetype id.** That's what keeps `switch archetype` out of the code (see [03-domain-model.md](03-domain-model.md)).

## One NPC, many conversations

```mermaid
flowchart LR
  NPC["NPC: Cô Ba<br/>one shared instance: pose, animation, behaviour"]
  S1["Session 81, private<br/>Lan (Traveler), node: chao_hoi"]
  S2["Session 82, private<br/>Minh (Citizen), node: mua_hang"]
  S3["Session 83, group<br/>Tuấn + Hoa, node: do_vui"]
  NPC --- S1
  NPC --- S2
  NPC --- S3
```

The three interaction modes:

- **Private:** only that player sees the dialogue.
- **Public:** the NPC's lines also appear as speech bubbles and in nearby chat, e.g. a vendor calling out.
- **Group:** several players share one position in the dialogue. Choices are made by the session leader, or by vote later.

The NPC turns toward whoever spoke last; that's visual only. NPC behaviour (standing, wandering, walking a route) runs in the world loop. The dialogue engine is a separate service, so an AI dialogue provider can replace scripted dialogue later without touching behaviour.

## How a quest step completes, with the server in charge

```mermaid
sequenceDiagram
  participant C as Client (Traveler)
  participant W as World loop
  participant B as Event bus
  participant Q as Quest engine (character worker)
  participant DB as SQLite
  C->>W: state {x,y,z,h,s} at 10 Hz
  W->>W: check speed, walkability, height on the nav grid
  W->>B: PlaceEntered(char, nha_tho_duc_ba) after a 2 s stay
  B->>Q: route to this character's worker
  Q->>Q: any active objective listening to place.entered?
  Q->>DB: one transaction: progress + rewards + reward_grants (unique key)
  DB-->>Q: commit
  Q-->>W: deliver quest.updated / reward
  W-->>C: quest.updated
  Note over C,Q: the client never says "objective done"; it only moves, talks, picks choices and uses items
```

## Ride-hailing (future): how it plugs in

The detailed plan, decided after phase 2, is [08-ride-hailing.md](08-ride-hailing.md): the trip states there replace the sketch below, and the core gains a few generic pieces (seat 1, carried players, view pins, a module hook, the wallet) rather than none.

```mermaid
stateDiagram-v2
  [*] --> Requested: passenger books a ride to a place in the Crab app
  Requested --> Offered: matcher notifies on-duty Crab Bike drivers nearby
  Offered --> Accepted: first driver to accept wins
  Offered --> Cancelled: timeout or passenger cancels
  Accepted --> PickingUp
  PickingUp --> OnTrip: passenger takes seat 1 within 3 m
  OnTrip --> Completed: bike stops inside the destination radius
  OnTrip --> Cancelled: passenger gets off early
  Completed --> Rated
  Rated --> [*]
  Cancelled --> [*]
```

It only needs pieces the MVP already builds:

- **Vehicle and seat entities.** The passenger's place is just seat 1 on the motorbike.
- **The `ridehail.drive` capability** lets a character go on duty; being on duty is a toggle in the Crab Driver app.
- **A phone notification type** routed to the Crab Driver app.
- **A `trip.completed` event** that quest objectives can listen for.

None of this changes the Player, movement sync, chat or the quest engine. "First driver wins" is a conditional update (see [04-backend.md](04-backend.md#transactions)).

## Runtime layout, now and later

```mermaid
flowchart LR
  subgraph Now["MVP: one VPS (today's deploy)"]
    NG["nginx :443"] --> GO["Go server: world + services"]
    GO --> SQ[("SQLite file in /var/lib/sg-tinymap")]
  end
  subgraph Later["Only when one process is not enough"]
    LB["nginx / load balancer"] --> G1["Go #1: Sài Gòn 1, 2"] & G2["Go #2: Sài Gòn 3, 4"]
    G1 & G2 --> PG[("Postgres, replacing SQLite")]
    G1 & G2 <--> RD[("Redis: presence, chat and DMs across processes, shared rate limits")]
  end
```

**Scaling plan:**

- Run several copies of the whole city ("Sài Gòn 1", "Sài Gòn 2") rather than splitting the 200 m map into zones.
- One process can host several copies: one world loop goroutine each, no Redis.
- The move to Postgres and Redis comes together, when a second process is needed.
