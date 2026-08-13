terraform {
  required_providers {
    laravel = {
      source  = "laravel/laravel"
      version = "~> 1.0"
    }
  }
}

provider "laravel" {
  # The API token may also be supplied via the LARAVEL_CLOUD_API_TOKEN
  # environment variable, which is preferred over committing it to config.
  # token = "..."

  # base_url defaults to https://cloud.laravel.com/api and rarely needs setting.
}
