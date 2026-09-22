terraform {
  required_providers {
    laravel = {
      source  = "laravel/laravel"
      version = "~> 1.0"
    }
  }
}

provider "laravel" {
  # Token via the LARAVEL_CLOUD_API_TOKEN env var.
  # base_url defaults to https://cloud.laravel.com/api
}
