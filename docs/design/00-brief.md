# The brief (condensed)

The original request from 2026-11-06, condensed to its requirements. The design that answers it is in the other files of this folder.

## Vision

- A multiplayer social virtual city based on Ho Chi Minh City. It combines open-world exploration, social interaction, role-play and life simulation, quests and progression, NPCs, and later player-driven activities and an economy.
- Recognizable landmarks, streets, buildings, cafés, markets and other local places.
- Runs on desktop and mobile web; lightweight and tuned for mobile browsers.
- **One shared world.** Character types are gameplay identities inside it, not separate modes or worlds.

## Archetypes (chosen when creating the character)

- **Traveler:** explores landmarks and places, talks to local NPCs, does tourist quests, learns Vietnamese culture and history, meets Travelers and Citizens.
  - About 5–6 quests lead to the certificate **"Officially Recognized as a Saigon Local"**. The exact quests come later; the framework must support this.
  - Possible rewards: badges, titles, unique outfits, access to special places or interactions, collectibles.
- **Citizen:** an ordinary resident. Explores, walks, runs, jumps, interacts, talks with players and NPCs, does general quests, joins social activities, discovers local places.
  - Vietnamese is the default for Citizen interactions.
  - Citizens can help Travelers discover the city, and get general and possibly Citizen-only quests.
  - Must not feel restricted or less interesting than the others.
- **Driver** (first called "GrabBike", now **Crab Bike**): walks and interacts like everyone else, owns or uses a motorbike, rides around the city and explores roads.
  - Later: receives ride requests on the in-game phone. A Traveler requests a ride to a destination, nearby drivers are notified, one accepts, picks the Traveler up, they ride together, and the trip completes.
  - **Ride-hailing is not part of the first implementation**, but it must fit in later without redesigning the player or character system.
  - Driver features must not be tightly coupled to the core player entity.

## Shared core (the same for everyone, built once)

- **Movement:** walk, run, jump, rotate, interact with objects, basic emotes, enter and exit special interactions. Must work on desktop and mobile web.
- **In-game phone,** designed as an extensible app platform.
  - First apps: Profile, Quest tracker, Notifications, Chat, Map, Achievements.
  - Later: driver app, ride-hailing, inventory, marketplace, social network, events, wallet.
  - Apps and permissions can differ by archetype: a driver app that receives ride requests, a tourist guide for Travelers, local community apps for Citizens.
  - Don't build every app now.
- **Chat:** global, nearby, and player-to-player. Shows the character's name and role, and styles messages by identity. Must work with hundreds of players at once. Needs moderation, spam prevention and rate limiting.
- **Character identity:** unique player id, character name, type, appearance, profile, progression, achievements, inventory, current location. Extensible, with **one user model for all archetypes**.

## Quests and progression

- A generic quest engine with no archetype hard-coded.
- Categories: main, character-specific, daily, side, exploration, social, event, NPC.
- Objective kinds: reach a location, interact with an NPC, talk to another player, collect an item, complete an activity, visit a landmark, a sequence of objectives.
- **Server-authoritative:** a manipulated client must not be able to fake completion.
- To design: quest definitions, objectives, prerequisites, states, progression, rewards, achievements. Decide whether definitions live in the database or in config files.
- Adding quests must not require changing core gameplay code.
- Examples given:
  - Traveler: visit Bến Thành Market, talk to a local NPC, try Vietnamese coffee, explore a historical landmark, a local culture challenge.
  - Citizen: help a Traveler, visit a local café, join community activities.
  - Driver: explore designated places; later, complete ride requests and help others get around.

## NPCs and interactions

- NPCs belong to the shared world. One NPC can talk with many players at the same time; **never one NPC copy per player**.
- To design: shared NPC state, per-player dialogue sessions, behaviour and animation, interaction range checks, the dialogue system, quest hooks, optional AI dialogue.
- Interaction modes: private dialogue, public conversation, group interaction.
- The dialogue engine is independent of NPC behaviour, so AI dialogue can come later. **An LLM never executes game transactions or grants rewards.**

## Archetype architecture

- Possible future types: tourist, citizen, motorbike driver, food delivery driver, business owner, street vendor, tour guide, photographer, police officer, event organizer.
- No large switch statements, no separate player implementation per type.
- A composable design: core player, archetype, capabilities/abilities, role modules, progression, unlockable features.
- Evaluate archetype + capabilities, composition-based modules, role configuration, or another pattern. Practical, not over-engineered.

## Technical context

- Backend in **Go**, already written. Don't default to Node.js/Colyseus; evaluate Go options first.
- Frontend: Three.js, web 3D, desktop and mobile web. Assets: GLB/glTF, a character animation system, a shared city map.
- Multiplayer: real-time movement over WebSocket, see nearby players, shared NPC state, spatial partitioning / area of interest, many players in the same city.
- Earlier direction (open to challenge): game zones or rooms, spatial AOI, server-authoritative gameplay, snapshot interpolation, Redis for shared presence or coordination.
- Modular backend, but no premature microservices.

## Extensibility

- Future features to keep room for: ride-hailing, motorbike ownership and customization, a player marketplace, a virtual economy, housing and property, business ownership, social events, group activities, AI NPCs, character customization, achievements, seasonal events.
- Adding an archetype must not require rewriting movement, multiplayer sync, the phone, chat, NPCs or the quest engine.
- Adding a phone app must not change the core player model.
- Adding a quest type needs a clearly defined extension point.

## Deliverables asked for

A. gameplay design · B. system architecture (with Mermaid diagrams) · C. domain model · D. Go backend architecture · E. frontend architecture · F. MVP scope (must / should / future) · G. roadmap (objectives, features, dependencies, acceptance criteria, risks per phase) · H. technical decisions and trade-offs.

## Working rules

- Read the repo first.
- Modular monolith first.
- Keep gameplay rules independent of transport and infrastructure.
- No archetype-specific logic in the Player entity.
- No unnecessary abstractions.
- Use concrete Saigon examples.
- Keep the MVP separate from future architecture.
- Challenge assumptions.
- **No implementation until the design is reviewed and agreed.**

## Answers given since (2026-11-06)

- Map: keep the ~200 m island for now; it may get bigger.
- Driver brand: **Crab Bike**, with a yellow uniform.
- Database: **SQLite** for now.
