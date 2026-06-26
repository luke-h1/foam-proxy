#!/usr/bin/env bash
#
# Keeps one environment's App Review magic link fresh. The app signs in with the
# baked access_token immediately and never refreshes on failure, so a stale token
# breaks the review login. Twitch user tokens last ~4h, so a cron runs this to
# refresh the test-account token and write the new blob to the GitHub secret
# (canonical) and the live Lambda env (immediate). One environment per run.
#
# Twitch binds a refresh token to its minting client id, so TWITCH_CLIENT_ID/SECRET
# must match the blob being refreshed.
#
# Required env:
#   MAGIC_LINK_BLOB       current blob JSON {access_token, refresh_token, ...}
#   TWITCH_CLIENT_ID      Twitch client id the blob was minted with
#   TWITCH_CLIENT_SECRET  Twitch client secret for that client id
#   LAMBDA_FUNCTION_NAME  e.g. foam-proxy-lambda-production / -staging
#   GH_TOKEN              PAT with "Secrets: write" to persist the rotated blob
# Optional env:
#   MAGIC_LINK_BLOB_SECRET  GitHub secret to write back to (default: MAGIC_LINK_BLOB)
set -euo pipefail

: "${MAGIC_LINK_BLOB:?MAGIC_LINK_BLOB is required}"
: "${TWITCH_CLIENT_ID:?TWITCH_CLIENT_ID is required}"
: "${TWITCH_CLIENT_SECRET:?TWITCH_CLIENT_SECRET is required}"
: "${LAMBDA_FUNCTION_NAME:?LAMBDA_FUNCTION_NAME is required}"
# Twitch rotates the refresh token on refresh, so the old one dies immediately. If
# we can't persist the new blob the next terraform apply reverts the Lambda to the
# dead token, so refuse to rotate rather than strand it.
: "${GH_TOKEN:?GH_TOKEN is required to persist the rotated blob}"
MAGIC_LINK_BLOB_SECRET="${MAGIC_LINK_BLOB_SECRET:-MAGIC_LINK_BLOB}"

echo "::add-mask::${TWITCH_CLIENT_SECRET}"

refresh_token=$(jq -r '.refresh_token // ""' <<<"${MAGIC_LINK_BLOB}")
token_type=$(jq -r '.token_type // "bearer"' <<<"${MAGIC_LINK_BLOB}")
[[ -n "${refresh_token}" ]] || { echo "MAGIC_LINK_BLOB has no refresh_token; cannot refresh" >&2; exit 1; }
echo "::add-mask::${refresh_token}"

# No -f: on an HTTP error curl -f drops the body, which under set -e aborts before
# the new_access check and hides Twitch's error. -S still surfaces connection failures.
response=$(curl -sS -X POST "https://id.twitch.tv/oauth2/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "client_id=${TWITCH_CLIENT_ID}" \
  --data-urlencode "client_secret=${TWITCH_CLIENT_SECRET}" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=${refresh_token}")

new_access=$(jq -r '.access_token // ""' <<<"${response}")
new_refresh=$(jq -r '.refresh_token // ""' <<<"${response}")
expires_in=$(jq -r '.expires_in // 0' <<<"${response}")
[[ -n "${new_access}" ]] || { echo "Twitch refresh failed: $(jq -rc '{status, message}' <<<"${response}" 2>/dev/null || echo "${response}")" >&2; exit 1; }
echo "::add-mask::${new_access}"

# Twitch usually rotates the refresh token; keep the old one only if it omits one.
if [[ -n "${new_refresh}" ]]; then
  echo "::add-mask::${new_refresh}"
else
  new_refresh="${refresh_token}"
fi

new_blob=$(jq -nc \
  --arg access_token "${new_access}" \
  --arg refresh_token "${new_refresh}" \
  --arg token_type "${token_type}" \
  --argjson expires_in "${expires_in}" \
  '{access_token: $access_token, refresh_token: $refresh_token, expires_in: $expires_in, token_type: $token_type}')

# 1) Persist the rotated blob to the canonical GitHub secret FIRST. If the Lambda
#    update fails, the secret still holds the fresh blob and the next apply/run
#    converges on it instead of reverting to the now-dead refresh token.
printf '%s' "${new_blob}" | gh secret set "${MAGIC_LINK_BLOB_SECRET}"
echo "Updated ${MAGIC_LINK_BLOB_SECRET} GitHub secret"

# 2) Update the live Lambda env for immediate effect, merging into the existing
#    variables so the rest (Twitch creds, DSNs) is preserved.
current_env=$(aws lambda get-function-configuration \
  --function-name "${LAMBDA_FUNCTION_NAME}" \
  --query 'Environment.Variables' --output json)
env_payload=$(jq -nc --argjson current "${current_env}" --arg blob "${new_blob}" \
  '{Variables: ($current + {MAGIC_LINK_BLOB: $blob})}')
aws lambda update-function-configuration \
  --function-name "${LAMBDA_FUNCTION_NAME}" \
  --environment "${env_payload}" >/dev/null

echo "Refreshed magic link token on ${LAMBDA_FUNCTION_NAME} (expires_in=${expires_in}s)"
