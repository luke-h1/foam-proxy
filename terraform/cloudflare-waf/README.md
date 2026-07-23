# Cloudflare WAF rules

Zone-level WAF rate limit for the `foam-app.com` zone, managed with Terraform.
This is a standalone stack, separate from the per-env AWS proxy stack in
`terraform/`, because WAF is zone-level: both `auth.foam-app.com` (production)
and `auth-staging.foam-app.com` share the one Cloudflare zone, so the rule
deploys once, not per environment. Both hosts are proxied (orange-clouded), so
Cloudflare sees their traffic.

## What it does

`cloudflare_ruleset.rate_limit` puts one per-source-IP rate limit on the
auth-proxy hosts in the `http_ratelimit` phase: **100 requests / 10s** per IP,
`block` on exceed, keyed on `ip.src` + `cf.colo.id`. It counts and blocks at the
Cloudflare edge, before a request reaches API Gateway or Lambda. The managed
`ddos_l7` ruleset already on the zone handles large volumetric floods.

### Free-plan limits

`foam-app.com` is on the Free plan, which caps rate limiting to **one rule** with
`period` and `mitigation_timeout` both fixed at **10s** (the API rejects any
other value). So there's no separate tighter limit on the token-minting paths
(`/api/token`, `/api/refresh-token`) - that needs a second rule and a paid plan.
The single rule covers every auth-host path, which is where a moderate scraper
would still cost Lambda invocations. `var.rate_limit_requests` is the only knob
that moves without a plan upgrade.

### Why 100/10s

A foam client hits this proxy lightly: a short login burst, one app-token fetch
per launch, a user-token refresh every few hours. 100/10s sits well above any
real user and stays generous because carrier CGNAT stacks many users behind one
egress IP and the per-IP counter can't tell them apart.

## State

S3 backend in the `foam-production-terraform-state` bucket under
`cloudflare-waf/`, with the `foam-proxy-production-terraform-state-lock`
DynamoDB lock table. The lock table is bootstrapped by the proxy deploy; the
bucket must already exist.

## Credentials

Loaded from 1Password by `.github/actions/load-cloudflare-waf-secrets`, from the
`foam-proxy-production` item in the `ci-cd` vault:

- `CLOUDFLARE_WAF_API_TOKEN` - token with **Zone.WAF:Edit** + **Zone.Zone:Read**
  on `foam-app.com`.
- `CLOUDFLARE_ZONE_ID` - the Cloudflare zone ID for `foam-app.com` (not the
  Route53 `ZONE_ID` GitHub secret).

AWS creds for the state bucket come from the existing `AWS_ACCESS_KEY_ID` /
`AWS_SECRET_ACCESS_KEY` GitHub secrets in the workflow.

## Deploy

`.github/workflows/terraform-cloudflare-waf.yml`: PRs touching this directory get
a `plan`; pushes to `main` (or a manual `workflow_dispatch`) `apply`.

Locally:

```sh
cd terraform/cloudflare-waf
export CLOUDFLARE_API_TOKEN=...          # Zone.WAF:Edit + Zone.Zone:Read
export TF_VAR_cloudflare_zone_id=...     # foam-app.com Cloudflare zone id
export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...
terraform init
terraform plan
```
