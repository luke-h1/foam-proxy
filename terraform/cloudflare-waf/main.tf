# Per-IP rate limit blocked at the Cloudflare edge, before a request reaches
# API Gateway/Lambda. Keyed on ip.src + cf.colo.id and scoped to the auth hosts
# so the rest of the zone is untouched. foam-app.com is on the Free plan, which
# permits one rate-limit rule with period and mitigation_timeout both fixed at
# 10s (verified against the live zone), so this is a single broad limit; the
# managed ddos_l7 ruleset already handles volumetric floods.

resource "cloudflare_ruleset" "rate_limit" {
  zone_id     = var.cloudflare_zone_id
  name        = "foam-proxy rate limit"
  description = "Per-IP edge rate limit for the auth-proxy hosts"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules = [{
    action      = "block"
    description = "Rate limit auth hosts per source IP"
    expression  = "(${join(" or ", formatlist("http.host eq \"%s\"", var.auth_hostnames))})"
    enabled     = true
    ratelimit = {
      characteristics     = ["ip.src", "cf.colo.id"]
      period              = var.rate_limit_period
      requests_per_period = var.rate_limit_requests
      mitigation_timeout  = var.rate_limit_period
    }
  }]
}
