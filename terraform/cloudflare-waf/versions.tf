terraform {
  required_version = ">= 1.10"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }

  # Zone-level WAF is a single control shared by every foam-app.com host, so it
  # lives in its own state, deployed once - not per-env like the AWS proxy stack.
  # Reuses the production state bucket + lock table the proxy already bootstraps.
  backend "s3" {
    bucket         = "foam-production-terraform-state"
    key            = "cloudflare-waf/terraform.tfstate"
    region         = "eu-west-2"
    dynamodb_table = "foam-proxy-production-terraform-state-lock"
    encrypt        = true
  }
}

# Auth via CLOUDFLARE_API_TOKEN env - needs Zone.WAF:Edit + Zone.Zone:Read on the
# foam-app.com zone. Loaded from 1Password in CI (see
# .github/actions/load-cloudflare-waf-secrets).
provider "cloudflare" {}
