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

# Creating this resource runs the command once on the environment, which must
# have been deployed. Changing the command runs the new one.
resource "laravel_cloud_command" "seed_roles" {
  environment_id = laravel_cloud_environment.example.id
  command        = "php artisan db:seed --class=RoleSeeder --force"
}

output "seed_roles_output" {
  value = laravel_cloud_command.seed_roles.output
}
