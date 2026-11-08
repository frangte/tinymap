# Deploying

`scripts/deploy.sh` builds everything on your machine and ships it to the server (`iris` in `~/.ssh/config`), where nginx already runs. Nothing is built on the server, and it needs neither Node nor Go.

```
browser ──https/wss :443──> nginx ──> /ws, /healthz ──> sg-tinymap (Go, systemd, 127.0.0.1:8787)
                              └──> everything else: files from <app>/current/dist
```

```bash
npm run deploy             # build, test, upload, activate       (scripts/deploy.sh)
npm run deploy:check       # what is on the server now, read-only (scripts/deploy.sh check)
npm run deploy:rollback    # back to the previous live release    (scripts/deploy.sh rollback)
scripts/deploy.sh --skip-tests
```

## What a deploy does

On your machine:

1. Checks that `ssh iris` works without a prompt, and asks the server for its CPU architecture.
2. Runs `npm run typecheck`, `go vet` and `go test -race` (skip with `--skip-tests`).
3. Runs `npm run build` and cross-compiles the server for `linux/<arch>`: a static binary.
4. Uploads a new release with rsync. Files unchanged since the current release become hard links, so only what changed travels.

On the server (`deploy/remote.sh`, streamed over ssh):

5. Installs the systemd unit and the nginx site, rendered from `deploy/systemd/app.service.template` and `deploy/nginx/site.conf.template`, but **only when they changed**. The nginx site goes through `nginx -t` first; if nginx rejects it, the old site is put back.
6. On the very first deploy, gets the TLS certificate (below).
7. Makes sure nginx (`www-data`) can pass through the home folders down to the game. Ubuntu home folders are 750, which makes nginx answer 403. This is `chmod o+x`, which lets it pass through but not list anything.
8. Switches `<app>/current` to the new release atomically.
9. Restarts `sg-tinymap` **only if the server binary or its unit changed**. A frontend-only deploy disconnects nobody. A restart drops players for a second or two; they reconnect by themselves and keep their characters. The chat history resets, because it lives in memory.
10. Waits for `/healthz`. If the new release doesn't answer, it goes back to the previous release together with the unit and nginx site it ran with, then stops with an error.
11. Marks the release as live and keeps the last 5 (`DEPLOY_KEEP`).

Finally, it checks `https://sg.tinymap.live/healthz` from your machine.

Layout on the server:

```
/home/ubuntu/apps/sg.tinymap.live/
  current -> releases/20261105-132842-793165e     the live one: nginx root and ExecStart go through it
  releases/<UTC time>-<git rev>/
    dist/               the built game
    bin/server          the Go server
    deploy/*.template   the configs this release was deployed with
    REVISION            git revision, build time, machine
    .activated          set once the release passed its health check (rollback targets only these)
/etc/systemd/system/sg-tinymap.service
/etc/nginx/sites-available/sg.tinymap.live.conf  (linked from sites-enabled)
```

The release name ends in `-dirty` when there were uncommitted changes, and the script warns about it.

## First deploy

1. **DNS:** an `A` record for `sg.tinymap.live` pointing at the server.
2. **Firewall** (Lightsail networking tab or EC2 security group): 80 and 443 open to everyone. **Keep 8787 closed.** The server listens on all interfaces, so the firewall is what keeps it private, and everything public goes through nginx.
3. **certbot** on the server, if `npm run deploy:check` says it's missing: `ssh iris sudo apt install -y certbot`.
4. Deploy with an email for Let's Encrypt:
   ```bash
   CERTBOT_EMAIL=you@example.com npm run deploy
   ```
   The script installs a temporary HTTP-only site and gets the certificate with `certbot certonly --webroot` (it never edits the nginx config). Then it installs the real site. certbot's own timer renews the certificate and reloads nginx.

Without `CERTBOT_EMAIL`, a first deploy stops before switching anything, with a message saying exactly this.

## Settings

Environment variables for `scripts/deploy.sh` (defaults in brackets):

| | |
|---|---|
| `DEPLOY_HOST` | ssh host (`iris`) |
| `DEPLOY_DIR` | app folder (`/home/ubuntu/apps/sg.tinymap.live`) |
| `DEPLOY_DOMAIN` | domain: nginx `server_name` and the certificate (`sg.tinymap.live`) |
| `DEPLOY_PORT` | local port of the Go server (`8787`). The deploy refuses to start if another program already uses it. |
| `DEPLOY_SERVICE` | systemd unit (`sg-tinymap`) |
| `DEPLOY_KEEP` | releases kept (`5`) |
| `CERTBOT_EMAIL` | first deploy only |

The Go server's own settings (`MAX_PLAYERS`, `LOG_LEVEL`, …, see `README.md`) are `Environment=` lines in `deploy/systemd/app.service.template`. Change them there and deploy; the changed unit triggers a restart.

## Database

The player database is one SQLite file, `/var/lib/sg-tinymap/game.db`. It lives outside `releases/` (which every deploy swaps and prunes), so deploys and rollbacks keep it. The unit's `StateDirectory=sg-tinymap` makes systemd create that folder, owned by the service user, and it is the only place the locked-down service may write.

- **Backups** are in `/var/lib/sg-tinymap/backup/`. The server takes one a day (`game-<UTC time>.db`, the last 7 kept), going by the newest file, so restarts don't skip it. It also takes one before any schema migration (`pre-v<N>-<time>.db`, kept until you delete them). Copy them off the machine now and then: `rsync -a iris:/var/lib/sg-tinymap/backup/ ./backups/`.
- **Restore:** `ssh iris sudo systemctl stop sg-tinymap`, then copy the backup over `game.db` and delete any `game.db-wal` / `game.db-shm` next to it. Then `sudo systemctl start sg-tinymap`.
- **Rollback** keeps the database. Migrations only add tables and columns, so the older server runs against the newer schema and ignores what it doesn't know.
- **Losing it** isn't fatal for players. Their stored tokens stop matching, the server answers `error auth`, and the game asks for a new guest account by itself.

## Day to day

```bash
npm run deploy:check                                   # release, service, certificate expiry, port, health
ssh iris journalctl -u sg-tinymap -f                   # join / leave lines, one log line per connection
ssh iris sudo tail -f /var/log/nginx/error.log
curl -s https://sg.tinymap.live/healthz                # {"ok":true,"players":3}
```

## Troubleshooting

| Symptom | Cause |
|---|---|
| `cannot ssh to 'iris' without a prompt` | the key in `~/.ssh/config` isn't loaded, or the host is down |
| `needs passwordless sudo` | the deploy user needs `NOPASSWD` sudo (the default `ubuntu` user on EC2/Lightsail has it) |
| `port 8787 is already used by another program` | another app on the server: `DEPLOY_PORT=8790 npm run deploy` |
| `nginx rejected the new site config` | the `nginx -t` output is printed above it; fix `deploy/nginx/site.conf.template` |
| Page 403 | nginx can't reach the files: the deploy fixes the folder permissions, so check that `DEPLOY_DIR` is right |
| Status stays "Đang kết nối…", `/ws` gives 403 | the page origin isn't the server's host: the page is served from another domain. Add `Environment=ALLOWED_ORIGINS=https://that-domain` to the unit template. |
| `/ws` gives 502 | the service is down: `ssh iris systemctl status sg-tinymap` |
| The service won't start, `database:` in `journalctl -u sg-tinymap` | it can't open or migrate `/var/lib/sg-tinymap/game.db`: a full disk, or the unit installed without `StateDirectory` (check `systemctl cat sg-tinymap`) |
| Healthy on the server, not from your machine | DNS, or ports 80/443 closed in the firewall |

## Tested

The whole flow runs against an Ubuntu 24.04 container standing in for `iris` (sshd, nginx 1.24, passwordless sudo, a 750 home folder). That covered:
- first deploy without a certificate
- full deploy
- a repeat deploy that skipped the restart
- a forced failing deploy that rolled back with its config
- manual rollback
- a real browser joining, walking, changing outfits and chatting through nginx over TLS

What the container can't cover: the real systemd (a stand-in was used), and certbot talking to Let's Encrypt.
