resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}
