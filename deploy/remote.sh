#!/usr/bin/env bash
# Runs ON the server: scripts/deploy.sh streams this file over ssh (`ssh host bash -s -- <command> ...`).
#
#   prepare  <app_dir>
#   activate <app_dir> <release> <domain> <port> <service> <keep> [certbot_email]
#   rollback <app_dir> <domain> <port> <service>
#   check    <app_dir> <domain> <port> <service>
#
# Layout:  <app_dir>/releases/<release>/{dist/, bin/server, deploy/*.template, REVISION}
#          <app_dir>/current -> releases/<release>     (nginx root and the systemd ExecStart go through it)
set -euo pipefail

log() { printf '   [server] %s\n' "$*"; }
die() { printf '   [server] error: %s\n' "$*" >&2; exit 1; }

cmd="${1:-}"
shift || true

need_sudo() {
  sudo -n true 2>/dev/null || die "this needs passwordless sudo for $(id -un)"
}

nginx_paths() {
  if [[ -d /etc/nginx/sites-available && -d /etc/nginx/sites-enabled ]]; then
    SITE="/etc/nginx/sites-available/$DOMAIN.conf"
    SITE_LINK="/etc/nginx/sites-enabled/$DOMAIN.conf"
  else
    SITE="/etc/nginx/conf.d/$DOMAIN.conf"
    SITE_LINK=""
  fi
  NGINX_USER="$(awk '$1 == "user" { gsub(";", "", $2); print $2; exit }' /etc/nginx/nginx.conf 2>/dev/null || true)"
  NGINX_USER="${NGINX_USER:-www-data}"
}

render() {
  sed -e "s#{{DOMAIN}}#$DOMAIN#g" -e "s#{{APP_DIR}}#$APP_DIR#g" -e "s#{{PORT}}#$PORT#g" -e "s#{{USER}}#$(id -un)#g" "$1"
}

# install_if_changed <new file> <destination>: 0 if it was installed, 1 if the destination already matched
install_if_changed() {
  if sudo test -f "$2" && sudo cmp -s "$1" "$2"; then return 1; fi
  if sudo test -f "$2"; then sudo cp -p "$2" "$2.bak"; fi
  sudo install -m 644 "$1" "$2"
  return 0
}

enable_site() {
  if [[ -n "$SITE_LINK" ]]; then sudo ln -sfn "$SITE" "$SITE_LINK"; fi
}

# nginx runs as another user: it must be able to pass through every directory down to the release
# (Ubuntu home directories are 750, which makes nginx answer 403)
ensure_traversable() {
  local d="$APP_DIR"
  while [[ "$d" != "/" ]]; do
    if ! sudo -u "$NGINX_USER" test -x "$d"; then
      sudo chmod o+x "$d"
      log "chmod o+x $d (so $NGINX_USER can reach the game files: pass through, not list)"
    fi
    d="$(dirname "$d")"
  done
}

port_busy() {
  ss -ltnH "( sport = :$PORT )" 2>/dev/null | grep -q .
}

switch_to() { # switch_to <release dir>: atomic swap of the `current` symlink
  ln -sfn "releases/$(basename "$1")" "$APP_DIR/current.new"
  mv -Tf "$APP_DIR/current.new" "$APP_DIR/current"
}

wait_healthy() {
  local i
  for i in $(seq 1 20); do
    if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then return 0; fi
    sleep 0.5
  done
  return 1
}

obtain_cert() {
  [[ -n "$EMAIL" ]] || die "no TLS certificate for $DOMAIN yet. Point its DNS at this server, then rerun with CERTBOT_EMAIL=you@example.com"
  command -v certbot >/dev/null || die "certbot is not installed: sudo apt install -y certbot"
  log "no certificate for $DOMAIN: requesting one from Let's Encrypt"
  sudo mkdir -p /var/www/letsencrypt
  local tmp
  tmp="$(mktemp)"
  # an HTTP-only site first: the full site refers to certificate files that do not exist yet
  cat >"$tmp" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN;
    location /.well-known/acme-challenge/ { root /var/www/letsencrypt; }
    location / { return 404; }
}
EOF
  install_if_changed "$tmp" "$SITE" || true
  rm -f "$tmp"
  enable_site
  sudo nginx -t
  sudo systemctl reload nginx
  sudo certbot certonly --webroot -w /var/www/letsencrypt -d "$DOMAIN" \
    --agree-tos -m "$EMAIL" --non-interactive --deploy-hook "systemctl reload nginx"
}

prune() {
  local current old
  current="$(basename "$(readlink -f "$APP_DIR/current")")"
  # names start with a UTC timestamp, so a plain sort is oldest first
  for old in $(ls -1 "$APP_DIR/releases" | sort | head -n "-$KEEP"); do
    [[ "$old" == "$current" ]] && continue
    rm -rf "${APP_DIR:?}/releases/$old"
    log "removed old release $old"
  done
}

case "$cmd" in
prepare)
  APP_DIR="$1"
  mkdir -p "$APP_DIR/releases"
  if [[ -e "$APP_DIR/current" ]]; then echo has-current; else echo first-deploy; fi
  ;;

activate)
  APP_DIR="$1" REL="$2" DOMAIN="$3" PORT="$4" SERVICE="$5" KEEP="$6" EMAIL="${7:-}"
  NEW="$APP_DIR/releases/$REL"
  [[ -x "$NEW/bin/server" && -f "$NEW/dist/index.html" ]] || die "release $REL is incomplete"
  need_sudo
  command -v nginx >/dev/null || die "nginx is not installed"
  nginx_paths
  PREV="$(readlink -f "$APP_DIR/current" 2>/dev/null || true)"
  if ! systemctl is-active --quiet "$SERVICE" && port_busy; then
    die "port $PORT is already used by another program: $(sudo ss -ltnpH "( sport = :$PORT )" | head -1). Deploy with DEPLOY_PORT=<free port>."
  fi

  # 1. systemd unit
  tmp="$(mktemp)"
  render "$NEW/deploy/app.service.template" >"$tmp"
  unit_changed=0
  if install_if_changed "$tmp" "/etc/systemd/system/$SERVICE.service"; then
    unit_changed=1
    sudo systemctl daemon-reload
    sudo systemctl enable --quiet "$SERVICE"
    log "installed systemd unit $SERVICE"
  fi

  # 2. TLS certificate (first deploy only)
  if ! sudo test -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem"; then obtain_cert; fi

  # 3. switch to the new release
  ensure_traversable
  switch_to "$NEW"
  log "current -> releases/$REL"

  # 4. nginx site
  render "$NEW/deploy/site.conf.template" >"$tmp"
  site_changed=0
  if install_if_changed "$tmp" "$SITE"; then
    site_changed=1
    enable_site
    if ! sudo nginx -t 2>"$tmp.nginx"; then
      cat "$tmp.nginx" >&2
      if sudo test -f "$SITE.bak"; then sudo mv "$SITE.bak" "$SITE"; else sudo rm -f "$SITE" "${SITE_LINK:-/nonexistent}"; fi
      [[ -n "$PREV" ]] && switch_to "$PREV"
      die "nginx rejected the new site config; the previous one is back"
    fi
    sudo systemctl reload nginx
    log "installed nginx site $SITE"
  fi
  rm -f "$tmp" "$tmp.nginx"

  # 5. restart the server only when it changed (a frontend-only deploy disconnects nobody)
  if [[ -z "$PREV" ]] || [[ "$unit_changed" == 1 ]] || ! cmp -s "$NEW/bin/server" "$PREV/bin/server" || ! systemctl is-active --quiet "$SERVICE"; then
    sudo systemctl restart "$SERVICE"
    log "restarted $SERVICE (players reconnect by themselves)"
  else
    log "server binary unchanged: no restart, nobody disconnected"
  fi

  # 6. health check, or back to the previous release together with the unit and site it ran with
  if ! wait_healthy; then
    sudo journalctl -u "$SERVICE" -n 30 --no-pager >&2 || true
    if [[ -n "$PREV" && "$PREV" != "$NEW" ]]; then
      unit="/etc/systemd/system/$SERVICE.service"
      if [[ "$unit_changed" == 1 ]] && sudo test -f "$unit.bak"; then
        sudo mv "$unit.bak" "$unit"
        sudo systemctl daemon-reload
      fi
      if [[ "$site_changed" == 1 ]] && sudo test -f "$SITE.bak"; then
        sudo mv "$SITE.bak" "$SITE"
        sudo nginx -t 2>/dev/null && sudo systemctl reload nginx
      fi
      switch_to "$PREV"
      sudo systemctl restart "$SERVICE"
      sleep 1
      if systemctl is-active --quiet "$SERVICE"; then
        die "the new release did not answer /healthz: rolled back to $(basename "$PREV") (and its config), which is running"
      fi
      die "the new release did not answer /healthz, and the rolled-back $(basename "$PREV") is not running either: see journalctl -u $SERVICE"
    fi
    die "the server did not answer /healthz (log above)"
  fi
  log "healthy: $(curl -fsS "http://127.0.0.1:$PORT/healthz")"
  # only releases that actually went live are rollback targets (a failed deploy can leave an uploaded one behind)
  touch "$NEW/.activated"
  prune
  ;;

rollback)
  APP_DIR="$1" DOMAIN="$2" PORT="$3" SERVICE="$4"
  need_sudo
  current="$(basename "$(readlink -f "$APP_DIR/current")")"
  # the newest release older than the current one that was live once (skips uploads whose deploy failed)
  prev=""
  for r in $(ls -1 "$APP_DIR/releases" | sort); do
    [[ "$r" == "$current" ]] && break
    [[ -f "$APP_DIR/releases/$r/.activated" ]] && prev="$r"
  done
  [[ -n "$prev" ]] || die "no earlier release that went live, nothing to roll back to"
  switch_to "$APP_DIR/releases/$prev"
  sudo systemctl restart "$SERVICE"
  wait_healthy || die "rolled back to $prev but it does not answer /healthz either"
  log "rolled back: $current -> $prev"
  ;;

check)
  APP_DIR="$1" DOMAIN="$2" PORT="$3" SERVICE="$4"
  nginx_paths
  echo "   host        $(hostname) ($(uname -m)), $(. /etc/os-release && echo "$PRETTY_NAME")"
  echo "   nginx       $(nginx -v 2>&1 | sed 's/^nginx version: //' || echo 'not installed')"
  echo "   certbot     $(command -v certbot >/dev/null && certbot --version 2>&1 || echo 'not installed (sudo apt install -y certbot)')"
  if sudo -n test -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" 2>/dev/null; then
    echo "   certificate $(sudo openssl x509 -enddate -noout -in "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" | sed 's/notAfter=/valid until /')"
  else
    echo "   certificate none yet for $DOMAIN (the first deploy needs CERTBOT_EMAIL)"
  fi
  echo "   nginx site  $([[ -f "$SITE" ]] && echo "$SITE" || echo 'not installed yet')"
  echo "   service     $SERVICE: $(systemctl is-active "$SERVICE" 2>/dev/null || true)"
  if [[ -e "$APP_DIR/current" ]]; then
    echo "   release     $(basename "$(readlink -f "$APP_DIR/current")") ($(cat "$APP_DIR/current/REVISION" 2>/dev/null || echo '?')), $(ls -1 "$APP_DIR/releases" | wc -l | tr -d ' ') kept"
  else
    echo "   release     none yet"
  fi
  if port_busy; then
    echo "   port $PORT   in use$(systemctl is-active --quiet "$SERVICE" && echo " by $SERVICE" || echo ' by ANOTHER program: deploy with DEPLOY_PORT=<free port>')"
  else
    echo "   port $PORT   free"
  fi
  echo "   health      $(curl -fsS --max-time 2 "http://127.0.0.1:$PORT/healthz" 2>/dev/null || echo 'no answer')"
  if sudo -n true 2>/dev/null; then echo "   sudo        ok (passwordless)"; else echo "   sudo        NEEDS a password: deploy will fail"; fi
  ;;

*)
  die "unknown command '$cmd' (prepare | activate | rollback | check)"
  ;;
esac
