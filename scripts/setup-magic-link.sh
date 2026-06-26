#!/usr/bin/env bash
#
# Generates the App Review magic link values for ONE environment and PRINTS them
# as JSON — it never writes to 1Password or GitHub (you store them). Run once per
# env. See README "App Review magic link" for the full flow.
#
#   (default)   --env <prod|staging>   Mint a Twitch user token for the disposable
#                                       test account, build the blob, and (prod
#                                       only) generate the shared gate key.
#   --verify    --env <prod|staging>   Fetch the live endpoint, confirm it returns a blob.
#   --teardown                         Revoke the Twitch token and, when
#                                       LAMBDA_FUNCTION_NAME is set, clear the live
#                                       Lambda env so the route 404s immediately.
#
# Prompts/guidance go to stderr so stdout stays pipeable, e.g.
#   scripts/setup-magic-link.sh --env prod -y | jq -r .magic_link_blob
#
# Prereqs: twitch, jq, shasum (setup); curl, jq (--verify); curl (--teardown,
#          plus aws + jq to clear the live Lambda env).
set -euo pipefail

# Mirror of the scopes in foam src/hooks/useTwitchSignIn.ts — keep in sync
# (one per line, same order, for easy diffing).
SCOPES=(
  user:read:follows
  user:read:blocked_users
  user:read:emotes
  user:manage:blocked_users
  chat:read
  chat:edit
  user:write:chat
  moderator:read:chat_messages
  moderator:manage:chat_messages
  whispers:read
  whispers:edit
  channel:read:polls
  channel:read:predictions
  channel:read:redemptions
  channel:moderate
  channel:manage:clips
  editor:manage:clips
)

# Shared gate key location (one key for both envs; prod setup mints it).
OP_REF="op://ci-cd/foam-staging/MAGIC_LINK_API_KEY"
EXPIRES_IN="${MAGIC_LINK_EXPIRES_IN:-14400}"

MODE="setup"
ENV_TARGET="" # prod|staging; required for setup and --verify
AUTO_YES=0

TOKEN_OUT=""
trap '[[ -n "${TOKEN_OUT}" ]] && rm -f "${TOKEN_OUT}"; true' EXIT

usage() {
  cat >&2 <<'EOF'
Usage: scripts/setup-magic-link.sh [--teardown | --verify] --env <prod|staging> [--yes]

Generates the magic-link blob and gate key for ONE env and prints JSON to stdout
(blob, gate key, review URL); it does not store them. Prompts go to stderr.

  (no flags)   Mint the token, build the blob, generate the gate key; print JSON.
  --verify     Fetch the live <env> ?format=json endpoint and confirm a blob.
  --teardown   Revoke the Twitch token; clears the live Lambda env when
               LAMBDA_FUNCTION_NAME is set (you still clear the stored secrets).
  --env <env>  Target environment: prod or staging (required for setup/--verify).
  --yes, -y    Don't prompt for confirmation.
EOF
}

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: '$1' is required but not installed" >&2; exit 1; }; }

confirm() {
  [[ "${AUTO_YES}" -eq 1 ]] && return 0
  local reply
  read -rp "$1 [y/N] " reply
  [[ "${reply}" =~ ^[Yy]([Ee][Ss])?$ ]]
}

base_url() {
  case "$1" in
    prod)    echo "https://auth.foam-app.com/api/magic" ;;
    staging) echo "https://auth-staging.foam-app.com/api/magic" ;;
    *)       return 1 ;;
  esac
}

# Opaque high-entropy secret: SHA-256 of 32 random bytes -> 64 hex chars.
gen_key() { head -c 32 /dev/urandom | shasum -a 256 | cut -d' ' -f1; }

parse_token() {
  local label="$1" text="$2"
  printf '%s\n' "${text}" | grep -i "${label}" | tail -n1 | sed -E "s/.*${label}:[[:space:]]*//" | tr -d '[:space:]' || true
}

mint_token() {
  local scope_str clean
  scope_str="${SCOPES[*]}" # default IFS joins with spaces

  cat >&2 <<EOF


EOF

  TOKEN_OUT="$(mktemp)"
  if ! twitch token -u --dcf -s "${scope_str}" 2>&1 \
       | tee "${TOKEN_OUT}" \
       | sed -E -e $'s/\x1b\\[[0-9;]*m//g' -e 's/(Token):.*/\1: [redacted]/' >&2; then
    echo "error: 'twitch token' failed" >&2
    return 1
  fi

  clean="$(sed -E $'s/\x1b\\[[0-9;]*m//g' "${TOKEN_OUT}")"
  ACCESS_TOKEN="$(parse_token 'User Access Token' "${clean}")"
  REFRESH_TOKEN="$(parse_token 'Refresh Token' "${clean}")"

  [[ -n "${ACCESS_TOKEN}"  ]] || { read -rsp "Could not parse access token — paste it: "  ACCESS_TOKEN;  echo; }
  [[ -n "${REFRESH_TOKEN}" ]] || { read -rsp "Could not parse refresh token — paste it: " REFRESH_TOKEN; echo; }
  [[ -n "${ACCESS_TOKEN}"  ]] || { echo "error: no access token" >&2; return 1; }
  [[ -n "${REFRESH_TOKEN}" ]] || { echo "error: no refresh token (need 'twitch token -u --dcf')" >&2; return 1; }

  echo "Parsed token (access …${ACCESS_TOKEN: -4}, refresh …${REFRESH_TOKEN: -4})." >&2
}

setup() {
  need twitch; need jq; need shasum
  local base; base="$(base_url "${ENV_TARGET}")" || { echo "error: setup requires --env prod|staging" >&2; exit 1; }

  local key blob_secret key_dest
  if [[ "${ENV_TARGET}" == "prod" ]]; then
    key="$(gen_key)"
    blob_secret="MAGIC_LINK_BLOB_PRODUCTION"
    key_dest="${OP_REF} — the shared gate key (both deploys read it)"
  else
    key="${MAGIC_LINK_API_KEY:-}"
    blob_secret="MAGIC_LINK_BLOB_STAGING"
    key_dest="reuses the shared ${OP_REF} from prod setup — no separate staging key"
  fi

  echo "Setting up the ${ENV_TARGET} magic link." >&2
  echo "The Twitch account must be a DISPOSABLE, low-privilege TEST account." >&2
  confirm "Signed in to / about to sign in as a throwaway test account?" \
    || { echo "Create one first, then re-run. Aborted." >&2; exit 1; }

  if confirm "Configure the Twitch CLI for ${ENV_TARGET} now? (it prompts for client id/secret)"; then
    twitch configure
  fi

  mint_token

  local blob
  blob="$(jq -nc \
    --arg access_token "${ACCESS_TOKEN}" \
    --arg refresh_token "${REFRESH_TOKEN}" \
    --argjson expires_in "${EXPIRES_IN}" \
    '{access_token: $access_token, refresh_token: $refresh_token, expires_in: $expires_in, token_type: "bearer"}')"

  cat >&2 <<EOF

============================================================================
Generated ${ENV_TARGET} values — NOT stored anywhere. Copy them in yourself:
  .magic_link_blob     -> GitHub secret '${blob_secret}'
  .magic_link_api_key  -> ${key_dest}

Next:
  - Verify once stored + deployed:
      MAGIC_LINK_API_KEY=${key} scripts/setup-magic-link.sh --verify --env ${ENV_TARGET}
  - Seed the first refresh:  gh workflow run refresh-magic-link.yml
  - After approval, revoke:  scripts/setup-magic-link.sh --teardown
============================================================================

EOF

  jq -n \
    --argjson blob "${blob}" \
    --arg key "${key}" \
    --arg env "${ENV_TARGET}" \
    --arg base "${base}" \
    '{
      env: $env,
      magic_link_blob: $blob,
      magic_link_api_key: $key,
      review_url: { browser: "\($base)?key=\($key)", json: "\($base)?key=\($key)&format=json" }
    }'
}

# verify fetches the live ?format=json endpoint and confirms a session blob. A 404
# means the key/blob has not reached the Lambda yet (deploy first) or the key is wrong.
verify() {
  need curl; need jq
  local base; base="$(base_url "${ENV_TARGET}")" || { echo "error: --verify requires --env prod|staging" >&2; exit 1; }

  local key="${MAGIC_LINK_API_KEY:-}"
  [[ -n "${key}" ]] || { read -rsp "MAGIC_LINK_API_KEY for ${ENV_TARGET}: " key; echo; }
  [[ -n "${key}" ]] || { echo "error: no key provided" >&2; exit 1; }

  echo "GET ${base}?key=…&format=json" >&2
  local resp; resp="$(curl -sS "${base}?key=${key}&format=json")"
  if jq -e 'has("access_token") and (.access_token | length > 0)' >/dev/null 2>&1 <<<"${resp}"; then
    echo "OK — ${ENV_TARGET} endpoint returned a session blob:" >&2
    # Mask the token: show only its prefix so it doesn't land in scrollback.
    jq '{access_token: (.access_token[0:6] + "…"), token_type, expires_in, has_refresh_token: has("refresh_token")}' <<<"${resp}"
  else
    echo "FAILED — no session blob (404 = key/blob not deployed yet, or wrong key):" >&2
    printf '%s\n' "${resp}" >&2
    exit 1
  fi
}

teardown() {
  need curl
  echo "Revoke the Twitch token, then disable the live route." >&2

  # Values taken interactively (env overrides for non-interactive use); nothing is
  # read from 1Password/GitHub.
  local client_id="${TWITCH_CLIENT_ID:-}" access="${MAGIC_LINK_ACCESS_TOKEN:-}"
  [[ -n "${client_id}" ]] || read -rp "  Twitch client id the token was minted with: " client_id
  [[ -n "${access}" ]] || { read -rsp "  Access token to revoke (empty to skip): " access; echo; }

  if [[ -n "${client_id}" && -n "${access}" ]]; then
    if confirm "  Revoke this token at Twitch?"; then
      curl -sS -X POST "https://id.twitch.tv/oauth2/revoke" \
        --data-urlencode "client_id=${client_id}" \
        --data-urlencode "token=${access}" >/dev/null && echo "  revoked." >&2
    fi
  else
    echo "  skipped token revoke (no client id / token)." >&2
  fi

  # The live route reads MAGIC_LINK_BLOB / MAGIC_LINK_API_KEY from the Lambda env at
  # runtime, so clearing the stored secrets alone leaves the URL working until the next
  # deploy. Strip them from the running Lambda now (merging into the existing variables
  # so the rest survives) for an immediate 404. Mirrors refresh-magic-link.sh.
  local lambda="${LAMBDA_FUNCTION_NAME:-}"
  if [[ -n "${lambda}" ]] && command -v aws >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
    if confirm "  Clear MAGIC_LINK_* from the live ${lambda} env now (immediate 404)?"; then
      local current_env env_payload
      current_env=$(aws lambda get-function-configuration \
        --function-name "${lambda}" --query 'Environment.Variables' --output json)
      [[ -n "${current_env}" && "${current_env}" != "null" ]] || current_env='{}'
      env_payload=$(jq -nc --argjson current "${current_env}" \
        '{Variables: ($current | del(.MAGIC_LINK_BLOB, .MAGIC_LINK_API_KEY))}')
      aws lambda update-function-configuration \
        --function-name "${lambda}" --environment "${env_payload}" >/dev/null
      echo "  cleared MAGIC_LINK_BLOB / MAGIC_LINK_API_KEY on ${lambda}; route now 404s." >&2
    fi
  else
    echo "  set LAMBDA_FUNCTION_NAME (with aws + jq installed) to clear the live env automatically." >&2
  fi

  cat >&2 <<EOF

Now finish teardown yourself:
  - Disconnect Foam from the test account: https://www.twitch.tv/settings/connections
  - GitHub    : delete MAGIC_LINK_BLOB_STAGING / MAGIC_LINK_BLOB_PRODUCTION (and MAGIC_LINK_PAT)
  - 1Password : clear ${OP_REF}
Clear the stored secrets too, or the next deploy re-populates the Lambda env and revives the route.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --teardown) MODE="teardown" ;;
    --verify)   MODE="verify" ;;
    --env)      ENV_TARGET="${2:-}"; shift ;;
    --env=*)    ENV_TARGET="${1#--env=}" ;;
    -y|--yes)   AUTO_YES=1 ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "error: unknown argument '$1'" >&2; usage; exit 1 ;;
  esac
  shift
done

case "${MODE}" in
  setup)    setup ;;
  teardown) teardown ;;
  verify)   verify ;;
esac
