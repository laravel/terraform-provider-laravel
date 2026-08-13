# Full-stack exercise of every resource the provider exposes.
#
# See README.md for how to run. The token is read from the
# LARAVEL_CLOUD_API_TOKEN env var (set by run.sh).
terraform {
  required_providers {
    laravel = {
      source  = "laravel/laravel"
      version = "~> 1.0"
    }
  }
}

provider "laravel" {
  # token comes from LARAVEL_CLOUD_API_TOKEN
  # base_url defaults to https://cloud.laravel.com/api
}
