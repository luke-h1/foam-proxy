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

The `golangci-lint` pre-commit hook runs via `go run`, so there is no separate
`golangci-lint` binary to install locally.

## Adding a Twitch App

The proxy selects a Twitch app from the `app` query param on requests
`/api/proxy?app=mobile` and `/api/token?app=desktop`.

Adding a new app currently requires code and Terraform changes.

1. Create the Twitch application in the Twitch developer console.
2. Choose the app key you want clients to send in the `app` query param.
3. Add that app key to `allowedApps` in `internal/config/config.go` with three env var names:
   `clientID`, `clientSecret`, and `redirectURI`.
4. Add matching Terraform variables in `terraform/variables.tf` for the new client ID, client secret, and redirect URI.
5. Expose those variables to the Lambda in `terraform/lambda.tf` by adding the corresponding environment variables.
6. Add the new app key to `proxy_apps` so it is included in `PROXY_APPS`.
7. Set the Terraform values for the target environment and deploy.
8. Update the calling client to pass the new `app` value on requests

The env var mapping for each app must stay consistent across all three places:
`internal/config/config.go`, `terraform/variables.tf`, and `terraform/lambda.tf`.

Example shape for a new app named `foam-desktop`:

```text
app query param: foam-desktop
client ID env var: TWITCH_CLIENT_ID_DESKTOP
client secret env var: TWITCH_CLIENT_SECRET_DESKTOP
redirect URI env var: REDIRECT_URI_DESKTOP
```
