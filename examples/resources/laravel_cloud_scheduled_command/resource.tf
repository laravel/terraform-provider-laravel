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

# Laravel Cloud creates the environment's app instance with it; type = "app"
# takes it over.
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.example.id
  name           = "App"
  type           = "app"
  size           = "flex-512mb"
  scaling_type   = "none"
}

# A command that runs on its own schedule, outside the Laravel scheduler.
resource "laravel_cloud_scheduled_command" "nightly_reports" {
  instance_id = laravel_cloud_instance.app.id
  name        = "Nightly reports"
  command     = "php artisan reports:send"
  schedule    = "0 3 * * *" # 03:00 UTC every day
}

resource "laravel_cloud_scheduled_command" "prune_models" {
  instance_id = laravel_cloud_instance.app.id
  name        = "Prune models"
  command     = "php artisan model:prune"
  schedule    = "@daily"
}
