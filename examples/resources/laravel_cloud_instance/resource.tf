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

  php_version  = "8.4:1"
  node_version = "22"

  uses_push_to_deploy = true
}

resource "laravel_cloud_instance" "example" {
  environment_id = laravel_cloud_environment.example.id
  name           = "web"
  size           = "flex-512mb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 1
}

# Laravel Cloud creates an app instance with every environment. type = "app"
# takes it over instead of creating another, and destroy leaves it to the
# environment.
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.example.id
  name           = "App"
  type           = "app"
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1

  uses_octane         = true
  hibernation_timeout = 0 # never hibernate
}
