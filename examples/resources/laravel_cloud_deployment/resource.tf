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

# Creating this resource deploys the environment's branch and waits for the
# deployment to finish. Replace it, for example with
# terraform apply -replace=laravel_cloud_deployment.example, to deploy again.
resource "laravel_cloud_deployment" "example" {
  environment_id = laravel_cloud_environment.example.id

  depends_on = [laravel_cloud_environment_variables.example]
}

# Deploy only once the environment has its configuration.
resource "laravel_cloud_environment_variables" "example" {
  environment_id = laravel_cloud_environment.example.id
  variables = {
    APP_ENV   = "production"
    LOG_LEVEL = "info"
  }
}
