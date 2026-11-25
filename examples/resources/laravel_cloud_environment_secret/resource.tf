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

variable "stripe_secret" {
  type      = string
  sensitive = true
}

# An organization secret, available to every environment it is attached to.
resource "laravel_cloud_secret" "stripe" {
  key   = "STRIPE_SECRET"
  value = var.stripe_secret
}

# Exposes STRIPE_SECRET to the production environment.
resource "laravel_cloud_environment_secret" "stripe" {
  environment_id = laravel_cloud_environment.example.id
  secret_id      = laravel_cloud_secret.stripe.id
}
