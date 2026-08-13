# ------------------------------------------------------------------
# Basic application
# ------------------------------------------------------------------

resource "laravel_cloud_application" "example" {
  name       = "my-first-app"
  repository = "laravel/laravel"
  region     = "us-east-2"

  # Required by the API from March 9, 2026.
  source_control_provider_type = "github"
}

# ------------------------------------------------------------------
# Production environment for the application
# ------------------------------------------------------------------

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"

  php_version  = "8.4:1"
  node_version = "22"

  uses_push_to_deploy = true
}

# ------------------------------------------------------------------
# Outputs
# ------------------------------------------------------------------

output "application_id" {
  value = laravel_cloud_application.example.id
}

output "environment_status" {
  value = laravel_cloud_environment.production.status
}

# php_version is sent as "8.4:1"; the API reports the major version here.
output "environment_php_major_version" {
  value = laravel_cloud_environment.production.php_major_version
}
