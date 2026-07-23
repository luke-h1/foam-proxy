terraform {
  required_version = ">= 1.10"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }


  backend "s3" {
    bucket         = "foam-production-terraform-state"
    key            = "cloudflare-waf/terraform.tfstate"
    region         = "eu-west-2"
    dynamodb_table = "foam-proxy-production-terraform-state-lock"
    encrypt        = true
  }
}

provider "cloudflare" {}
