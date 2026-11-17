# foam-proxy

Lambda which supports authentication proxying for [foam](https://github.com/luke-h1/foam) app

## Tech stack

- [AWS Lambda](https://aws.amazon.com/lambda/)
- [Terraform](https://www.terraform.io/)
- [Changesets](https://github.com/changesets/changesets) for release notes and versions


## Run locally

The Lambdas are standard `aws-lambda-go` handlers, run locally under the AWS Lambda
Runtime Interface Emulator (needs Docker).

```bash
cp .env.local.example .env.local   # fill in TWITCH_CLIENT_ID/SECRET etc.
make run-local                     # starts the proxy on :9000 (leave running)
```

In another shell, POST an event fixture from `events/`:

```bash
make invoke-local                              # defaults to events/proxy-healthcheck.json
make invoke-local EVENT=events/proxy-version.json
```

Run a different Lambda with `CMD` (both targets):

```bash
make CMD=authorizer run-local
make CMD=authorizer invoke-local EVENT=events/authorizer.json
```

For pure logic changes the tests are the faster loop:
`TWITCH_CLIENT_ID=x TWITCH_CLIENT_SECRET=y go test ./...`

## Changelog and version

Release notes are changesets. A pull request adds a `.changeset/*.md` file with
`bun run changeset`: a bump type and one or two sentences for the person who
runs or calls the proxy. The `Changeset present` check fails a pull request
that adds no file. Use `bun run changeset -- --empty` for a change with nothing
to tell a user. See `.changeset/README.md` for the headings.

The production deploy (merge to `main`) cuts the release. It reads the highest
bump type in the pending notes, applies it to the latest `X.Y.Z` tag, writes
the notes into `CHANGELOG.md` under that version, deletes the files, tags the
commit and creates the GitHub release. A merge with only empty notes deploys
but cuts no release. The git tags are the source of truth for the version.

Preview the notes that are waiting with `make changelog` or
`bun run release:notes pending`.

Commit messages follow conventional commits:

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

- `magic_link_blob` → GitHub secret `MAGIC_LINK_BLOB_<ENV>` — store the raw JSON only (`{"access_token":…}`), **no surrounding quotes**. The secret is passed verbatim into SSM as `TF_VAR_magic_link_blob`; wrapping quotes get stored literally, break `ParseMagicLink`, and make `/api/magic` 404.
- `magic_link_api_key` → 1Password `op://ci-cd/foam-proxy-<env>/MAGIC_LINK_API_KEY` (distinct per env)
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
