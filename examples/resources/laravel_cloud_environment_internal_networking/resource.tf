# Internal networking is for environments on a dedicated (Private Cloud)
# cluster.
data "laravel_cloud_dedicated_clusters" "apps" {
  region = "us-east-2"
  type   = "applications"
  status = "active"
}

locals {
  cluster_id = data.laravel_cloud_dedicated_clusters.apps.clusters[0].id
}

resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
  cluster_id                   = local.cluster_id
}

resource "laravel_cloud_environment" "backend" {
  application_id = laravel_cloud_application.example.id
  name           = "backend"
  branch         = "main"
  cluster_id     = local.cluster_id
}

resource "laravel_cloud_environment" "api" {
  application_id = laravel_cloud_application.example.id
  name           = "api"
  branch         = "main"
  cluster_id     = local.cluster_id
}

# Let only the API environment reach the backend over the private network.
resource "laravel_cloud_environment_internal_networking" "backend" {
  environment_id  = laravel_cloud_environment.backend.id
  access_level    = "specific"
  environment_ids = [laravel_cloud_environment.api.id]
}

# The API reaches the backend at this hostname.
resource "laravel_cloud_environment_variables" "api" {
  environment_id = laravel_cloud_environment.api.id
  variables = {
    BACKEND_URL = "http://${laravel_cloud_environment_internal_networking.backend.hostname}"
  }
}
