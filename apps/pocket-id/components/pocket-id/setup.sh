#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    exit 1
  fi
}

require_env POD_NAMESPACE
require_env POCKET_ID_INTERNAL_URL
require_env POCKET_ID_DEPLOYMENT
require_env POCKET_ID_BOOTSTRAP_SECRET
require_env POMERIUM_NAMESPACE
require_env POMERIUM_SECRET
require_env POMERIUM_CLIENT_ID
require_env POMERIUM_CALLBACK_URL
require_env POMERIUM_LAUNCH_URL

secret_key_exists() {
  local namespace="$1"
  local secret="$2"
  local key="$3"
  kubectl -n "${namespace}" get secret "${secret}" -o "jsonpath={.data.${key}}" 2>/dev/null | grep -q .
}

pomerium_secret_ready() {
  secret_key_exists "${POMERIUM_NAMESPACE}" "${POMERIUM_SECRET}" client_id &&
    secret_key_exists "${POMERIUM_NAMESPACE}" "${POMERIUM_SECRET}" client_secret
}

wait_for_pocket_id() {
  for _ in $(seq 1 60); do
    if curl -fsS "${POCKET_ID_INTERNAL_URL}/healthz" >/dev/null; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for Pocket ID health endpoint" >&2
  return 1
}

restart_pocket_id() {
  kubectl -n "${POD_NAMESPACE}" rollout restart "deployment/${POCKET_ID_DEPLOYMENT}" >/dev/null
  kubectl -n "${POD_NAMESPACE}" rollout status "deployment/${POCKET_ID_DEPLOYMENT}" --timeout=180s
  wait_for_pocket_id
}

cleanup_static_api_key() {
  if kubectl -n "${POD_NAMESPACE}" get secret "${POCKET_ID_BOOTSTRAP_SECRET}" >/dev/null 2>&1; then
    kubectl -n "${POD_NAMESPACE}" delete secret "${POCKET_ID_BOOTSTRAP_SECRET}" >/dev/null
    restart_pocket_id
  fi
}

ensure_static_api_key() {
  local api_key
  api_key="$(kubectl -n "${POD_NAMESPACE}" get secret "${POCKET_ID_BOOTSTRAP_SECRET}" -o jsonpath='{.data.apiKey}' 2>/dev/null | base64 -d || true)"
  if [[ -n "${api_key}" ]]; then
    printf '%s' "${api_key}"
    return 0
  fi

  api_key="$(openssl rand -base64 32 | tr -d '\n')"
  kubectl -n "${POD_NAMESPACE}" create secret generic "${POCKET_ID_BOOTSTRAP_SECRET}" \
    --from-literal="apiKey=${api_key}" \
    --dry-run=client \
    -o yaml | kubectl apply -f - >/dev/null
  printf '%s' "${api_key}"
}

client_payload() {
  jq -n \
    --arg id "${POMERIUM_CLIENT_ID}" \
    --arg callback "${POMERIUM_CALLBACK_URL}" \
    --arg launch "${POMERIUM_LAUNCH_URL}" \
    '{
      id: $id,
      name: "Pomerium",
      callbackURLs: [$callback],
      logoutCallbackURLs: [],
      isPublic: false,
      pkceEnabled: false,
      requiresReauthentication: false,
      credentials: { federatedIdentities: [] },
      launchURL: $launch,
      isGroupRestricted: false
    }'
}

upsert_pomerium_client() {
  local api_key="$1"
  local payload status
  payload="$(client_payload)"
  status="$(curl -sS -o /tmp/pocket-id-client.json -w "%{http_code}" \
    -H "X-API-Key: ${api_key}" \
    "${POCKET_ID_INTERNAL_URL}/api/oidc/clients/${POMERIUM_CLIENT_ID}")"

  case "${status}" in
    200)
      curl -fsS -X PUT \
        -H "X-API-Key: ${api_key}" \
        -H "Content-Type: application/json" \
        -d "${payload}" \
        "${POCKET_ID_INTERNAL_URL}/api/oidc/clients/${POMERIUM_CLIENT_ID}" >/dev/null
      ;;
    404)
      curl -fsS -X POST \
        -H "X-API-Key: ${api_key}" \
        -H "Content-Type: application/json" \
        -d "${payload}" \
        "${POCKET_ID_INTERNAL_URL}/api/oidc/clients" >/dev/null
      ;;
    *)
      echo "unexpected Pocket ID client lookup status: ${status}" >&2
      cat /tmp/pocket-id-client.json >&2 || true
      return 1
      ;;
  esac
}

write_pomerium_secret() {
  local api_key="$1"
  local client_secret
  client_secret="$(curl -fsS -X POST \
    -H "X-API-Key: ${api_key}" \
    "${POCKET_ID_INTERNAL_URL}/api/oidc/clients/${POMERIUM_CLIENT_ID}/secret" | jq -r '.secret')"

  if [[ -z "${client_secret}" || "${client_secret}" == "null" ]]; then
    echo "Pocket ID did not return a client secret" >&2
    return 1
  fi

  kubectl -n "${POMERIUM_NAMESPACE}" create secret generic "${POMERIUM_SECRET}" \
    --from-literal="client_id=${POMERIUM_CLIENT_ID}" \
    --from-literal="client_secret=${client_secret}" \
    --dry-run=client \
    -o yaml | kubectl apply -f - >/dev/null
}

if pomerium_secret_ready; then
  cleanup_static_api_key
  echo "Pomerium OIDC client secret already exists"
  exit 0
fi

api_key="$(ensure_static_api_key)"
restart_pocket_id
upsert_pomerium_client "${api_key}"
write_pomerium_secret "${api_key}"
cleanup_static_api_key
echo "Pomerium OIDC client bootstrap complete"
