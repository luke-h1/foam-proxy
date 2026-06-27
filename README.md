# foam-proxy

Lambda which supports authentication proxying for [foam](https://github.com/luke-h1/foam) app

## Tech stack

- [AWS Lambda](https://aws.amazon.com/lambda/)
- [Terraform](https://www.terraform.io/)
- [git-cliff](https://github.com/orhun/git-cliff) for changelog and version bumps


## Changelog & version

Changelog is generated from conventional commits. On push to `main`, the **Changelog** workflow updates `CHANGELOG.md` and commits it.

```bash
uv tool install pre-commit
uv tool install commitizen
pre-commit install
pre-commit install --hook-type commit-msg
```

## App Review magic link keepalive

The `/api/magic` route serves a session token to App Store reviewers. Its blob lives in SSM and is rotated automatically by the scheduled `magic-keepalive` Lambda. Use `scripts/setup-magic-link.sh` (run `-h` for flags) — it only prints values, you store them.

**Add** (one env at a time):

```bash
scripts/setup-magic-link.sh --env <prod|staging>   # mints token, prints blob + gate key
```

- `magic_link_blob` → GitHub secret `MAGIC_LINK_BLOB_<ENV>`
- `magic_link_api_key` → 1Password `op://ci-cd/foam-proxy-<env>/MAGIC_LINK_API_KEY` (same key value in both envs for time being)
- Run **Deploy `<env>`** with `reviewer_account_refresh_enabled = true` (seeds SSM, serves `/api/magic`, starts the schedule), then verify:

```bash
MAGIC_LINK_API_KEY=<key> scripts/setup-magic-link.sh --verify --env <prod|staging>
```

**Update**: the keepalive Lambda refreshes the token on a schedule — no action needed. To rotate manually (e.g. account/scopes changed), re-run `--env <env>`, overwrite the GitHub secret, and re-deploy.

**Remove**: re-run **Deploy `<env>`** with `reviewer_account_refresh_enabled = false` (tears down the SSM blob + schedule, 404s the route), then:

```bash
scripts/setup-magic-link.sh --teardown
```

Finally delete the stored secrets (GitHub `MAGIC_LINK_BLOB_*`, 1Password gate keys) or the next deploy revives the route.
