variable "env" {
  type        = string
  description = "The environment to deploy to"
}

variable "twitch_client_id" {
  type = string
}

variable "twitch_client_secret" {
  type = string
}

# variable "api_key" {
#   type = string
# }

variable "zone_id" {
  type        = string
  description = "The zone id for the route53 record"
}

variable "root_domain" {
  type        = string
  description = "The root domain for the route53 record"
  default     = "lhowsam.com"
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
}
variable "deployed_by" {
  type        = string
  description = "Who initiated the deployment?"
}
