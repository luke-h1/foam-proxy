output "rate_limit_ruleset_id" {
  value       = cloudflare_ruleset.rate_limit.id
  description = "ID of the zone rate-limit ruleset."
}
