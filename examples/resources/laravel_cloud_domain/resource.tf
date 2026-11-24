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

# Redirect www.example.com to example.com.
resource "laravel_cloud_domain" "example" {
  environment_id = laravel_cloud_environment.example.id
  name           = "example.com"
  www_redirect   = "www_to_root"
}

# The DNS records to create at your DNS provider before the domain verifies.
output "domain_dns_records" {
  value = laravel_cloud_domain.example.dns_records
}

# An app subdomain. Once its records in dns_records exist, for example after a
# first apply with wait_for_verification = false, apply waits for it to verify.
resource "laravel_cloud_domain" "app" {
  environment_id        = laravel_cloud_environment.example.id
  name                  = "app.example.com"
  verification_method   = "real_time"
  wait_for_verification = true
  verification_timeout  = 15
}
