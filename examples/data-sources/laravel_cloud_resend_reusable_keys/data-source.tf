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
