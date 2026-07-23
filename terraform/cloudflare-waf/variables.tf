variable "cloudflare_zone_id" {
  description = "Zone ID for foam-app.com."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{32}$", var.cloudflare_zone_id))
    error_message = "cloudflare_zone_id must be a 32-character hex zone id."
  }
}

variable "auth_hostnames" {
  description = "Auth-proxy hostnames the rate limit is scoped to."
  type        = list(string)
  default     = ["auth.foam-app.com", "auth-staging.foam-app.com"]

  validation {
    condition     = length(var.auth_hostnames) > 0 && alltrue([for h in var.auth_hostnames : endswith(h, ".foam-app.com")])
    error_message = "auth_hostnames must be non-empty and every host must be within the foam-app.com zone."
  }
}

variable "rate_limit_requests" {
  description = "Max requests per source IP per window."
  type        = number
  default     = 100

  validation {
    condition     = var.rate_limit_requests > 0 && floor(var.rate_limit_requests) == var.rate_limit_requests
    error_message = "rate_limit_requests must be a positive integer."
  }
}

variable "rate_limit_period" {
  description = "Rate-limit window in seconds; also used as the block duration. Free plan only permits 10."
  type        = number
  default     = 10

  validation {
    condition     = contains([10, 60, 120, 300, 600], var.rate_limit_period)
    error_message = "rate_limit_period must be one of 10, 60, 120, 300, or 600 (only 10 is valid on the Free plan)."
  }
}
