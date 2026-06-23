set -eu

log() {
  printf '%s forgejo setup: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"
}

forgejo_admin() {
  timeout "${FORGEJO_ADMIN_TIMEOUT:-30}" \
    forgejo --work-path "$FORGEJO_WORK_PATH" --config "$FORGEJO_CONFIG_FILE" admin "$@"
}

log "waiting for Forgejo admin CLI"
ready=false
for attempt in $(seq 1 "${FORGEJO_READY_ATTEMPTS:-60}"); do
  if forgejo_admin auth list >/tmp/forgejo-auth-list.out 2>/tmp/forgejo-auth-list.err; then
    ready=true
    log "Forgejo admin CLI is ready on attempt ${attempt}"
    break
  fi

  log "Forgejo admin CLI not ready on attempt ${attempt}; retrying"
  sed -n '1,20p' /tmp/forgejo-auth-list.err
  sleep "${FORGEJO_READY_SLEEP_SECONDS:-5}"
done

if [ "$ready" != "true" ]; then
  log "Forgejo admin CLI did not become ready"
  cat /tmp/forgejo-auth-list.err
  exit 1
fi

auth_id="$(awk '$2 == "PocketID" { print $1; exit }' /tmp/forgejo-auth-list.out)"
if [ -n "$auth_id" ]; then
  log "updating PocketID OAuth source ${auth_id}"
  forgejo_admin auth update-oauth \
    --id "$auth_id" \
    --name PocketID \
    --provider openidConnect \
    --key "$OIDC_CLIENT_ID" \
    --secret "$OIDC_CLIENT_SECRET" \
    --auto-discover-url "$OIDC_DISCOVERY_URL" \
    --scopes openid \
    --scopes profile \
    --scopes email
  log "PocketID OAuth source updated"
  exit 0
fi

log "adding PocketID OAuth source"
forgejo_admin auth add-oauth \
  --name PocketID \
  --provider openidConnect \
  --key "$OIDC_CLIENT_ID" \
  --secret "$OIDC_CLIENT_SECRET" \
  --auto-discover-url "$OIDC_DISCOVERY_URL" \
  --scopes openid \
  --scopes profile \
  --scopes email
log "PocketID OAuth source added"
