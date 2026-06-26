# foam-proxy

Lambda which supports authentication proxying for [foam](https://github.com/luke-h1/foam) app

## Tech stack

- [AWS Lambda](https://aws.amazon.com/lambda/)
- [Terraform](https://www.terraform.io/)
- [git-cliff](https://github.com/orhun/git-cliff) for changelog and version bumps

## App Review magic link

`GET /api/magic?key=<secret>` is a backdoor for Apple App Review. The app requires
a Twitch login with 2FA, which reviewers can't complete, so this redirects into
the app with a stored test-account session (`foam://auth?access_token=…&refresh_token=…`),
bypassing OAuth. The route is public but gated by a constant-time secret check —
a wrong/missing key returns `404` so the route never reveals itself.

Appending `&format=json` returns the raw session blob instead of the redirect page,
behind the same key check. The blob always carries `access_token`, `refresh_token`
and `token_type` (defaulting to `bearer`); `expires_in` is included only when known.
This lets an in-app App Review login fetch the token and inject it directly (via
`loginWithTwitch`) rather than bouncing through the `foam://auth` deep link.

### Targeting a build variant (`&scheme=`)

By default the magic link redirects to `foam://auth`. Because a custom scheme is
shared across whatever apps claim it, that opens whichever build the OS picks
(usually production). Append `&scheme=<variant>` to redirect into a specific build
that registers that scheme instead:

```
GET /api/magic?key=<secret>&scheme=foam-internal   → foam-internal://auth?access_token=…
```

Only allowlisted schemes are accepted (`foam`, `foam-dev`, `foam-internal`,
`foam-testflight`, `foam-e2e` — see `allowedAppSchemes` in `internal/config`);
anything else falls back to `foam`, so the route can't be abused to bounce a token
into an arbitrary scheme. Omitting it preserves the production default, which is
what App Review uses. The override is magic-link only — the interactive OAuth
routes (`/api/proxy`, `/api/pending`) always target `foam`.

The `?key` secret lives in its own `MAGIC_LINK_API_KEY` env var — one shared gate
key for both environments, sourced from `op://ci-cd/foam-staging/MAGIC_LINK_API_KEY`
during deploy — kept separate from the token data so the gate secret is decoupled
from the rotating blob. The session lives in a per-environment GitHub secret —
`MAGIC_LINK_BLOB_STAGING` or `MAGIC_LINK_BLOB_PRODUCTION` — a JSON blob:

```json
{ "access_token": "…", "refresh_token": "…", "expires_in": 14400, "token_type": "bearer" }
```

Each env has its own blob, minted against that env's Twitch app and refreshed
independently — Twitch binds a refresh token to its minting client id.

### Setup

Run `scripts/setup-magic-link.sh --env <prod|staging>` once per environment (needs
the `twitch`, `gh`, `op`, `jq` CLIs). It mints the token and prints the blob, gate
key and review URL as JSON; you store them. Per env:

1. Create a disposable, low-privilege Twitch test account (never a personal one).
2. Run the script for the env — it points the Twitch CLI at that env's app and
   mints a user token with all of the app's scopes (Device Code Flow, so you get a
   `refresh_token`). The gate key is generated for you (`MAGIC_LINK_API_KEY` is just
   an opaque high-entropy string).
3. Store `.magic_link_blob` in the env's GitHub secret (`MAGIC_LINK_BLOB_STAGING` /
   `MAGIC_LINK_BLOB_PRODUCTION`). The gate key is shared: prod setup also yields
   `.magic_link_api_key` — store it once in `op://ci-cd/foam-staging/MAGIC_LINK_API_KEY`
   (both deploys read it); staging reuses it.
4. Set the `MAGIC_LINK_PAT` GitHub secret to a fine-grained PAT with **Secrets:
   write** (the refresh cron uses it to rotate the blobs).
5. Hand Apple the `.review_url.browser` value in App Review notes.
6. After approval, revoke the token (`--teardown`) and clear the secrets.

### Staying fresh

The app signs in with the baked `access_token` immediately and does **not** refresh
on failure, so a stale token breaks the review login. The **Refresh magic link**
workflow (`.github/workflows/refresh-magic-link.yml`) runs every 3h with one job per
environment: each refreshes its blob via the `refresh_token`, persists the new blob
to that env's GitHub secret (canonical, so a later `terraform apply` stays fresh) and
then updates the live Lambda env (immediate). Twitch rotates the refresh token on each
refresh, so the write-back needs `MAGIC_LINK_PAT`; without it the run fails rather than
rotating a token it can't persist. Run it manually after storing a blob to seed the
first refresh.

## Changelog & version

Changelog is generated from conventional commits. On push to `main`, the **Changelog** workflow updates `CHANGELOG.md` and commits it.

```bash
uv tool install pre-commit
uv tool install commitizen
pre-commit install
pre-commit install --hook-type commit-msg
```
