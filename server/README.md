# Sài Gòn Q1 server

Go server for multiplayer: [gin](https://github.com/gin-gonic/gin) for HTTP, [gorilla/websocket](https://github.com/gorilla/websocket) for the socket. One shared world, capped at 40 players. Everyone sees everyone else walk, what they wear, and the chat.

```bash
npm run server          # from the repo root: go run ./cmd/server on :8787
npm run server:test     # go vet + go test -race
```

In dev, run `npm run server` next to `npm run dev`. Vite forwards `/ws` to `localhost:8787`, so the game connects to `ws://<page host>/ws` both in dev and in production.

Production is one process: build the game and let the server serve it.

```bash
npm run build
cd server && go build -o bin/server ./cmd/server
STATIC_DIR=../dist ./bin/server
```

Deploying to the server (nginx + systemd, `npm run deploy`): see [DEPLOY.md](DEPLOY.md).

## Configuration (environment)

| Variable | Default | |
|---|---|---|
| `PORT` | `8787` | listen port |
| `STATIC_DIR` | (none) | serve this directory (the Vite build) for every path other than `/ws` and `/healthz` |
| `ALLOWED_ORIGINS` | (none) | extra page origins allowed to open the socket, comma-separated, exact `scheme://host[:port]` (`https://tinymap.live,https://www.tinymap.live`). Use it when the page is served from another domain. A page on the server's own host is always allowed. |
| `TRUSTED_PROXIES` | none | reverse proxy IPs/CIDRs whose `X-Forwarded-For` gin trusts |
| `MAX_PLAYERS` | `40` | player cap; the next one gets `{"t":"error","code":"full"}` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

`GET /healthz` returns `{"ok":true,"players":3}`.

## Layout

```
cmd/server/main.go          config from env, graceful shutdown
internal/web/router.go      gin routes, WebSocket upgrade, origin check, static files
internal/hub/hub.go         the world, owned by one goroutine (no locks); snapshots at 12 Hz
internal/hub/client.go      per-connection read/write loops, ping/pong, rate limits
internal/protocol/msg.go    message types and validation (the rules below)
```

## Protocol

JSON text frames, one message per frame, the type in `t`. The client mirrors this in `src/net/protocol.ts`. Positions are three.js metres (Y up) and the heading is radians (0 = facing +Z), which matches `Player.position` and `Player.heading` in the game.

`look` is `{"body": "man"|"woman", "outfit": "thun"|"aodai"|"baba"|"somi"|"xeom", "hat": "nonla"|"cap"|"none"}` (`src/game/outfits.ts`).

### Client to server

| Message | When | Rules |
|---|---|---|
| `{"t":"hello","name":"Lan","look":{…}}` | first frame, within 10 s | an invalid look or a different first frame → `error bad_hello`, then the socket closes. An empty name becomes `Khách <id>`. |
| `{"t":"state","x":1.2,"y":0.18,"z":-3.4,"h":3.14}` | about 10 Hz while playing | \|x\|, \|z\| ≤ 400, −20 ≤ y ≤ 120, all finite; anything else is dropped. Up to 20/s; extra frames are dropped silently. |
| `{"t":"profile","name":"Lan","look":{…}}` | after the dressing panel | same rules as hello; at most 4/s |
| `{"t":"chat","text":"xin chào"}` | | cleaned (below), 1–140 characters; about one per 0.7 s with a burst of 3, otherwise `error rate` and the line is dropped |

Names and chat are cleaned on the server: control characters and invisible format characters (zero-width, bidi overrides) are removed, runs of whitespace (newlines too) become one space, and the result is trimmed and capped (16 characters for names, 140 for chat). Frames over 1 KB close the connection. Malformed or unknown messages are dropped; the connection stays open.

### Server to client

| Message | When |
|---|---|
| `{"t":"welcome","id":7,"name":"Lan","max":40,"players":[player…],"chat":[line…]}` | reply to hello. `players` is everyone else, `chat` the last 30 lines, oldest first. `name` is the cleaned name. |
| `{"t":"join","p":player}` | someone else joined |
| `{"t":"leave","id":7}` | someone left, timed out, or was dropped |
| `{"t":"profile","id":7,"name":"Lan","look":{…}}` | someone else changed name or look |
| `{"t":"snap","at":1730790000000,"p":[[id,x,y,z,h,age],…]}` | at most 12 Hz, only when someone moved. Includes every placed player, the receiver too (skip your own id). Rounded to cm and milliradians. `age` is how many ms before this snapshot the server received that pose: place it at (receive time − age) to follow the sender's own timing rather than the snapshot tick. |
| `{"t":"chat","id":7,"name":"Lan","text":"xin chào","at":1730790000000}` | a chat line, sent to everyone including its author |
| `{"t":"error","code":"full"\|"bad_hello"\|"rate"}` | `full` and `bad_hello` are followed by the server closing the socket |

`player` is `{"id":7,"name":"Lan","look":{…},"pose":[x,y,z,h]}`. `pose` is missing until that player has sent its first state.

The server pings every 54 s and drops a connection that hasn't answered within 60 s. A client whose send queue falls 64 messages behind is dropped too (`leave` for everyone else).
