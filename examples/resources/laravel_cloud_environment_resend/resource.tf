resource "laravel_cloud_environment_resend" "example" {
  environment_id = laravel_cloud_environment.example.id
  from_address   = "hello@example.com"
  from_name      = "Example"
}
