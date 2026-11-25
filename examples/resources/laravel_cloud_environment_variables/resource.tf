resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "example" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"
  php_version    = "8.4:1"
}

# The keys listed here are written on every apply, and a key removed from the
# map is deleted from the environment. Variables set outside Terraform are
# left alone. Keep secrets in laravel_cloud_secret instead.
resource "laravel_cloud_environment_variables" "example" {
  environment_id = laravel_cloud_environment.example.id

  variables = {
    APP_NAME    = "My App"
    APP_ENV     = "production"
    APP_DEBUG   = "false"
    APP_URL     = "https://example.com"
    LOG_CHANNEL = "stderr"
    LOG_LEVEL   = "warning"
  }
}
