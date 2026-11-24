# A Laravel application. Laravel Cloud creates its default environment,
# "production", with it; manage that environment with laravel_cloud_environment.
resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

# A static site, such as a Vite or Astro build, served straight from its
# build output without an application server.
resource "laravel_cloud_application" "docs" {
  name                         = "my-docs"
  repository                   = "my-org/my-docs"
  region                       = "us-east-2"
  source_control_provider_type = "github"

  serving_mode  = "static"
  build_command = "npm ci && npm run build"
  serving_root  = "dist"
}
