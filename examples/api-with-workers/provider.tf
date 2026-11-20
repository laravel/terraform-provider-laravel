terraform {
  required_providers {
    laravel = {
      source  = "laravel/laravel"
      version = "~> 1.0"
    }
  }
}

provider "laravel" {
  # Set the token in the LARAVEL_CLOUD_API_TOKEN environment variable.
  # base_url defaults to https://cloud.laravel.com/api
}
