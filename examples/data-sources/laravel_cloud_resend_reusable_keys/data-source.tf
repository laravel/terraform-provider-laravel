resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.example.id
  name           = "staging"
  branch         = "develop"
}

# The sending keys of the organization's other environments that staging can
# reuse.
data "laravel_cloud_resend_reusable_keys" "staging" {
  environment_id = laravel_cloud_environment.staging.id
}

# Send from staging with the key production already uses.
resource "laravel_cloud_environment_resend" "staging" {
  environment_id = laravel_cloud_environment.staging.id
  from_address   = "staging@example.com"
  from_name      = "Example Staging"
  reuse_key_id = one([
    for k in data.laravel_cloud_resend_reusable_keys.staging.keys : k.id if k.name == "production"
  ])
}
