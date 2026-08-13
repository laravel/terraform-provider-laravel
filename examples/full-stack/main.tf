# ===========================================================================
# Tier 1 -- core resources (13)
#
# Plain CRUD resources that a default `apply` creates. The four
# side-effecting resources (deployment / command / snapshot / restore) live
# in side-effecting.tf, gated behind var.enable_side_effecting.
#
# Attribute values mirror the plan-check tests in
# internal/provider/resources_plan_test.go, which is the source of truth for
# what each resource accepts. If the API rejects a region / type / size,
# tweak it here.
# ===========================================================================

# --- Application + environment -------------------------------------------

resource "laravel_cloud_application" "app" {
  name       = "${var.prefix}-app"
  repository = "laravel/laravel"
  region     = var.app_region

  # Required by the API from March 9, 2026.
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "13.x" # laravel/laravel's real default branch (not "main")

  php_version  = "8.4:1"
  node_version = "22"

  uses_push_to_deploy = true
}

# --- Compute: instance + background process ------------------------------

resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
}

# Not available on every deployment -- off by default so a plain `apply`
# stays green. Enable with -var enable_api_blocked=true. See README.
resource "laravel_cloud_background_process" "worker" {
  count = var.enable_api_blocked ? 1 : 0

  instance_id = laravel_cloud_instance.web.id
  type        = "worker"
  processes   = 2
}

# --- Environment config: variables + domain ------------------------------

resource "laravel_cloud_environment_variables" "vars" {
  environment_id = laravel_cloud_environment.env.id
  variables = {
    APP_ENV   = "production"
    LOG_LEVEL = "debug"
  }
}

# Gated by the same variable as background_process, above.
resource "laravel_cloud_domain" "domain" {
  count = var.enable_api_blocked ? 1 : 0

  environment_id = laravel_cloud_environment.env.id
  name           = var.domain_name
}

# --- Database cluster + database -----------------------------------------

resource "laravel_cloud_database_cluster" "cluster" {
  name   = "${var.prefix}-db"
  type   = "laravel_mysql_84"
  region = "us-east-2"
  # The API requires the full config_schema for laravel_mysql_84:
  # size (enum), storage (5-1000 GB), is_public, uses_scheduled_snapshots,
  # retention_days (0-30). Discover via GET /databases/types.
  #
  # Every key the API echoes back must be present here, and with the exact
  # value the API stores. `config` is an opaque JSON string, so Read writes the
  # server's version straight into state -- any key we omit (or spell
  # differently) shows up as a diff on the next plan, and the resulting PATCH
  # fails with "An update operation is already in progress" while the cluster
  # is still provisioning. Two traps, both hit in practice:
  #   - size must be a database size from the enum ("mysql-flex-512mb"), NOT an
  #     instance size like "db-flex.m-1vcpu-512mb". The API accepts the wrong
  #     value silently and normalizes it, so the mistake only surfaces as a
  #     permanent diff.
  #   - suspend_seconds is optional on create but always returned, so pin it.
  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 7
    suspend_seconds          = 0
  })
}

resource "laravel_cloud_database" "db" {
  cluster_id = laravel_cloud_database_cluster.cluster.id
  name       = "app_db"
}

# --- Cache ----------------------------------------------------------------

# NOTE: type "upstash_redis" is currently un-createable on dev -- the API both
# requires is_public AND rejects it for that type (a backend catch-22). Use
# laravel_valkey, which accepts is_public=false. See GET /caches/types for
# types/sizes/regions.
resource "laravel_cloud_cache" "cache" {
  name                 = "${var.prefix}-valkey"
  type                 = "laravel_valkey"
  region               = var.app_region
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false
}

# --- Storage bucket + key -------------------------------------------------

resource "laravel_cloud_storage_bucket" "bucket" {
  name     = "${var.prefix}-assets"
  key_name = "primary"
}

resource "laravel_cloud_storage_bucket_key" "key" {
  bucket_id  = laravel_cloud_storage_bucket.bucket.id
  name       = "deploy-key"
  permission = "read_write"
}

# --- Websocket server + application --------------------------------------

resource "laravel_cloud_websocket_server" "server" {
  name            = "${var.prefix}-reverb"
  type            = "reverb"
  region          = "us-east-2"
  max_connections = 5000 # enum: 100, 200, 500, 2000, 5000, 10000 (1000 is invalid)
}

resource "laravel_cloud_websocket_application" "wsapp" {
  server_id        = laravel_cloud_websocket_server.server.id
  name             = "chat"
  allowed_origins  = ["https://example.com"]
  ping_interval    = 30
  activity_timeout = 60 # API constraint: activity_timeout must be 1-60 seconds
}

# --- Outputs (computed fields read back from the API) --------------------

output "application_id" {
  value = laravel_cloud_application.app.id
}

output "environment_id" {
  value = laravel_cloud_environment.env.id
}

output "environment_status" {
  value = laravel_cloud_environment.env.status
}

output "environment_php_major_version" {
  value = laravel_cloud_environment.env.php_major_version
}

output "database_cluster_status" {
  value = laravel_cloud_database_cluster.cluster.status
}

output "cache_status" {
  value = laravel_cloud_cache.cache.status
}

output "storage_bucket_endpoint" {
  value = laravel_cloud_storage_bucket.bucket.endpoint
}

output "websocket_server_hostname" {
  value = laravel_cloud_websocket_server.server.hostname
}
