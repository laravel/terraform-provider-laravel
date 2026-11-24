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

# Send email from the environment through Resend. The sender's domain must be
# verified in Resend; the laravel_cloud_resend_domains data source lists them.
resource "laravel_cloud_environment_resend" "example" {
  environment_id = laravel_cloud_environment.example.id
  from_address   = "hello@example.com"
  from_name      = "Example"
}
