variable "cloudflare_zone_id" {
  description = "Zone ID for foam-app.com."
  type        = string
}

variable "auth_hostnames" {
  description = "Auth-proxy hostnames the rate limit is scoped to."
  type        = list(string)
  default     = ["auth.foam-app.com", "auth-staging.foam-app.com"]
}

variable "rate_limit_requests" {
  description = "Max requests per source IP per window."
  type        = number
  default     = 100
}

variable "rate_limit_period" {
  description = "Rate-limit window in seconds; also used as the block duration. Free plan only permits 10."
  type        = number
  default     = 10
}
