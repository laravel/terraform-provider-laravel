resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

resource "laravel_cloud_database_cluster" "example" {
  name    = "my-app-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"
  config = jsonencode({
    size                     = "mysql-flex-1gb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = true
    retention_days           = 7
  })
}

resource "laravel_cloud_database" "production" {
  cluster_id = laravel_cloud_database_cluster.example.id
  name       = "my_app"
}

resource "laravel_cloud_cache" "example" {
  name                 = "my-app-cache"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false
}

# Laravel Cloud creates the application's "production" environment with the
# application, so this takes it over instead of creating another.
resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"

  php_version  = "8.4:1"
  node_version = "22"

  build_command  = "composer install --no-dev --optimize-autoloader && npm ci && npm run build"
  deploy_command = "php artisan migrate --force"

  uses_push_to_deploy = true
  timeout             = 30 # seconds

  database_id = laravel_cloud_database.production.id
  cache_id    = laravel_cloud_cache.example.id

  response_headers_frame        = "sameorigin"
  response_headers_hsts_max_age = 31536000
}

# A staging environment that deploys the develop branch and asks search
# engines not to index it.
resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.example.id
  name           = "staging"
  branch         = "develop"
  php_version    = "8.4:1"
  color          = "orange"

  uses_push_to_deploy = true

  response_headers_robots_tag = "noindex, nofollow"
}
