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
