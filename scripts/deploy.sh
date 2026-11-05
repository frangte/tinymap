#!/usr/bin/env bash
# Deploy Sài Gòn Q1 (the built game + the Go server) to the server: build here, upload a release, switch to it.
#
#   scripts/deploy.sh                 build, test, upload, activate (the default)
#   scripts/deploy.sh --skip-tests    same, without typecheck / go test
#   scripts/deploy.sh check           what is on the server now (read-only)
#   scripts/deploy.sh rollback        back to the previous release
#
# First deploy: point the domain's DNS at the server, then run with CERTBOT_EMAIL=you@example.com to get the
# TLS certificate. Later deploys need nothing. Settings (environment, defaults below):
#   DEPLOY_HOST     ssh host (from ~/.ssh/config)                       iris
#   DEPLOY_DIR      app folder on the server                            /home/ubuntu/apps/sg.tinymap.live
#   DEPLOY_DOMAIN   public domain (nginx server_name, certificate)      sg.tinymap.live
#   DEPLOY_PORT     local port of the Go server (nginx proxies to it)   8787
#   DEPLOY_SERVICE  systemd unit name                                   sg-tinymap
#   DEPLOY_KEEP     releases kept for rollback                          5
#   CERTBOT_EMAIL   Let's Encrypt account email, first deploy only
# Written for the bash 3.2 that ships with macOS.
set -euo pipefail

HOST="${DEPLOY_HOST:-iris}"
APP_DIR="${DEPLOY_DIR:-/home/ubuntu/apps/sg.tinymap.live}"
DOMAIN="${DEPLOY_DOMAIN:-sg.tinymap.live}"
PORT="${DEPLOY_PORT:-8787}"
SERVICE="${DEPLOY_SERVICE:-sg-tinymap}"
KEEP="${DEPLOY_KEEP:-5}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE_SH="$ROOT/deploy/remote.sh"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

cmd=deploy
skip_tests=0
for arg in "$@"; do
  case "$arg" in
  deploy | check | rollback) cmd="$arg" ;;
  --skip-tests) skip_tests=1 ;;
  -h | --help) usage; exit 0 ;;
  *) die "unknown argument '$arg' (see --help)" ;;
  esac
done

# remote <command> [args...]: run deploy/remote.sh on the server, arguments quoted for its shell
remote() {
  local quoted
  quoted="$(printf '%q ' "$@")"
  ssh "$HOST" "bash -s -- $quoted" <"$REMOTE_SH"
}

preflight() {
  local tool
  for tool in ssh rsync "$@"; do
    command -v "$tool" >/dev/null || die "'$tool' is not installed"
  done
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" true 2>/dev/null ||
    die "cannot ssh to '$HOST' without a prompt (check ~/.ssh/config and the key)"
}

case "$cmd" in
check)
  preflight
  log "$HOST: $APP_DIR"
  remote check "$APP_DIR" "$DOMAIN" "$PORT" "$SERVICE"
  if curl -fsS --max-time 10 "https://$DOMAIN/healthz" >/dev/null 2>&1; then
    echo "   public      https://$DOMAIN/healthz answers"
  else
    echo "   public      https://$DOMAIN/healthz does not answer (yet)"
  fi
  exit 0
  ;;
rollback)
  preflight
  log "rolling back $DOMAIN on $HOST"
  remote rollback "$APP_DIR" "$DOMAIN" "$PORT" "$SERVICE"
  exit 0
  ;;
esac

# ---- deploy
preflight npm go
cd "$ROOT"

arch="$(ssh "$HOST" uname -m)"
case "$arch" in
aarch64 | arm64) goarch=arm64 ;;
x86_64 | amd64) goarch=amd64 ;;
*) die "unsupported server architecture '$arch'" ;;
esac

rev="$(git rev-parse --short HEAD 2>/dev/null || echo nogit)"
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
  warn "uncommitted changes will be deployed (release is marked $rev-dirty)"
  rev="$rev-dirty"
fi
release="$(date -u +%Y%m%d-%H%M%S)-$rev"

if [[ "$skip_tests" == 0 ]]; then
  log "tests: typecheck, go vet, go test"
  npm run --silent typecheck
  (cd server && go vet ./... && go test -race -count=1 ./... >/dev/null)
fi

log "building the game (vite) and the server (linux/$goarch)"
npm run --silent build >/dev/null
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
chmod 755 "$stage" # mktemp makes it 700, and rsync -a would copy that onto the release folder
mkdir -p "$stage/bin" "$stage/deploy"
cp -R dist "$stage/dist"
(cd server && GOOS=linux GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$stage/bin/server" ./cmd/server)
cp deploy/nginx/site.conf.template deploy/systemd/app.service.template "$stage/deploy/"
printf '%s built %s on %s\n' "$rev" "$(date -u +%FT%TZ)" "$(hostname -s)" >"$stage/REVISION"

state="$(remote prepare "$APP_DIR")"
link=()
# unchanged files (the 2 MB model, most of dist/) become hard links to the previous release instead of uploads
if [[ "$state" == has-current ]]; then link=(--link-dest="$APP_DIR/current/"); fi
log "uploading release $release to $HOST:$APP_DIR/releases/"
rsync -az --chmod=Dgo+rx,Fgo+r ${link[@]+"${link[@]}"} "$stage/" "$HOST:$APP_DIR/releases/$release/"

log "activating"
remote activate "$APP_DIR" "$release" "$DOMAIN" "$PORT" "$SERVICE" "$KEEP" "$CERTBOT_EMAIL"

if curl -fsS --max-time 10 "https://$DOMAIN/healthz" >/dev/null 2>&1; then
  log "live: https://$DOMAIN  ($(curl -fsS --max-time 10 "https://$DOMAIN/healthz"))"
else
  warn "the server is healthy, but https://$DOMAIN/healthz does not answer from here: check DNS and that ports 80/443 are open in the instance firewall"
fi
