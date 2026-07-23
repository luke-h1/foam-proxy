variable "cloudflare_zone_id" {
  description = "Zone ID for foam-app.com. Set via TF_VAR_cloudflare_zone_id (loaded from 1Password in CI)."
  type        = string
}

# Prod + staging auth hosts, both in the foam-app.com zone. The rule matches
# these exactly so the rest of the zone is untouched.
variable "auth_hostnames" {
  description = "Auth-proxy hostnames the rate limit is scoped to."
  type        = list(string)
  default     = ["auth.foam-app.com", "auth-staging.foam-app.com"]
}

# 100 req / 10s per IP (~10 req/s). A foam client hits this proxy lightly (a
# short login burst, an app-token fetch per launch, a refresh every few hours),
# so this sits well above any real user; kept generous because carrier CGNAT
# stacks many users behind one egress IP.
variable "rate_limit_requests" {
  description = "Max requests per source IP per window."
  type        = number
  default     = 100
}

# The Free plan pins both the window and the block duration to 10s (the API
# rejects anything else). Raising this needs a paid Cloudflare plan.
variable "rate_limit_period" {
  description = "Rate-limit window in seconds; also used as the block duration. Free plan only permits 10."
  type        = number
  default     = 10
}
