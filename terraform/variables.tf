variable "env" {
  type        = string
  description = "The environment to deploy to"
}
variable "app_twitch_client_id" {
  type        = string
  description = "The Twitch client ID for foam-app. Maps to `TWITCH_CLIENT_ID_APP`."
}

variable "app_twitch_client_secret" {
  type        = string
  description = "The Twitch client secret for foam-app. Maps to `TWITCH_CLIENT_SECRET_APP`."
}

variable "app_redirect_uri" {
  type        = string
  description = "The redirect URI for foam-app. Maps to `REDIRECT_URI_FOAM_APP`."
  default     = "foam://"
}

variable "menubar_twitch_client_id" {
  type        = string
  description = "The Twitch client ID for foam-menubar. Maps to `TWITCH_CLIENT_ID_MENUBAR`."
}

variable "menubar_twitch_client_secret" {
  type        = string
  description = "The Twitch client secret for foam-menubar. Maps to `TWITCH_CLIENT_SECRET_MENUBAR`."
}

variable "menubar_redirect_uri" {
  type        = string
  description = "The redirect URI for foam-menubar. Maps to `REDIRECT_URI_MENUBAR`."
  default     = "foammenubar://"
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

variable "proxy_apps" {
  type        = string
  description = "The list of apps that the proxy handles. Maps to `PROXY_APPS`."
  default     = "foam-app, foam-menubar"
}

variable "pushgateway_url" {
  type        = string
  description = "Pushgateway base URL"
}

variable "pushgateway_auth_header" {
  type        = string
  description = "Optional auth header to send to the Pushgateway ingress"
  sensitive   = true
}
