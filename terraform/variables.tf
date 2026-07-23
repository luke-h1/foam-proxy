variable "env" {
  type        = string
  description = "The environment to deploy to"
}

variable "twitch_client_id" {
  type        = string
  description = "the twitch client id to use"
}

variable "twitch_client_secret" {
  type        = string
  description = "the twitch client secret to use"
}

variable "zone_id" {
  type        = string
  description = "The zone id for the route53 record"
}

variable "root_domain" {
  type        = string
  description = "The root domain for the route53 record"
  default     = "foam-app.com"
}

variable "private_key" {
  type        = string
  description = "The private key for the certificate"
}

variable "certificate_body" {
  type        = string
  description = "The certificate body for the certificate"
}

variable "certificate_chain" {
  type        = string
  description = "The certificate chain for the certificate"
}

variable "tags" {
  type        = map(string)
  description = "The tags to apply to the resources"
  default = {
    "Service"   = "FoamProxy"
    "ManagedBy" = "Terraform"
  }
}

variable "git_sha" {
  type        = string
  description = "the git sha that triggered the deployment"
  default     = "change-me"
}
variable "deployed_by" {
  type        = string
  description = "Who initiated the deployment?"
  default     = "luke-h1"
}

variable "project_name" {
  type        = string
  description = "name of the project"
  default     = "foam-proxy"
}

variable "api_key" {
  type        = string
  description = "the API key to set on the authorizer"
  sensitive   = true
}

variable "authorizer_dsn" {
  type        = string
  description = "the dsn of the authorizer sentry project"
  sensitive   = true
}

variable "proxy_dsn" {
  type        = string
  description = "the dsn of the authorizer sentry project"
  sensitive   = true
}

variable "magic_link_blob" {
  type        = string
  description = "JSON blob for the App Review magic link: {access_token, refresh_token, expires_in, token_type}. Empty disables the magic link."
  default     = ""
  sensitive   = true
}

variable "magic_link_api_key" {
  type        = string
  description = "Secret the magic route's ?key is compared against. Held separately from the magic_link_blob token data. Empty disables the magic link."
  default     = ""
  sensitive   = true
}

variable "reviewer_account_refresh_enabled" {
  type        = bool
  description = "Master on/off switch for the App Review magic link + token keepalive. When true the blob is stored in SSM, the proxy serves /api/magic, and the scheduled magic-keepalive Lambda refreshes the token; enabling also requires magic_link_blob to be seeded. When false the SSM blob is torn down, /api/magic 404s, and the keepalive schedule is disabled — so nothing runs and SSM is never touched while reviewers aren't looking."
  default     = false
}

variable "discord_webhook_url" {
  type        = string
  description = "The discord webhook URL used for receiving alarm notifications"
  default     = ""
  sensitive   = true
}

variable "telegram_bot_token" {
  type        = string
  description = "The telegram bot token"
  default     = ""
  sensitive   = true
}

variable "telegram_chat_id" {
  type        = string
  description = "The telegram chat ID which should receive alarm notifications"
  default     = ""
  sensitive   = true
}
