# ===========================================================================
# Acme Invoicing -- a multi-tenant SaaS with a production and a staging
# environment behind one application.
#
# The shape here is the one most teams land on: both environments track the
# same repository on different branches, share a database cluster but never a
# database, and only production gets a custom domain, a cache and autoscaling.
# ===========================================================================

resource "laravel_cloud_application" "invoicing" {
  name       = "acme-invoicing"
  repository = var.repository
  region     = var.region

  # Required by the API from March 9, 2026.
  source_control_provider_type = "github"

  slack_channel = "#deploys"
}

# --- Shared data services --------------------------------------------------

# One cluster, one database per environment. Sharing the cluster keeps the
# bill down; separate databases keep a staging migration from touching
# production data.
resource "laravel_cloud_database_cluster" "primary" {
  name    = "acme-invoicing-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = var.region

  # `config` is an opaque JSON document whose shape depends on the database
  # type. Every key the API returns must be present, with exactly the value it
  # stores, or the next plan shows a permanent diff. Discover the schema and
  # the valid `size` enum with the laravel_cloud_database_types data source.
  config = jsonencode({
    size                     = "mysql-flex-1gb"
    storage                  = 25
    is_public                = false
    uses_scheduled_snapshots = true
    retention_days           = 14
    suspend_seconds          = 0
  })
}

resource "laravel_cloud_database" "production" {
  cluster_id = laravel_cloud_database_cluster.primary.id
  name       = "invoicing_production"
}

resource "laravel_cloud_database" "staging" {
  cluster_id = laravel_cloud_database_cluster.primary.id
  name       = "invoicing_staging"
}

# Production alone gets a cache. Staging uses the database session/cache
# drivers, which is one less thing to pay for.
resource "laravel_cloud_cache" "production" {
  name                 = "acme-invoicing-cache"
  type                 = "laravel_valkey"
  region               = var.region
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false
  eviction_policy      = "allkeys-lru"
}

# Tenant-uploaded PDFs. The bucket creates its own initial key; the second key
# is the narrower one the invoice renderer gets.
resource "laravel_cloud_storage_bucket" "invoices" {
  name       = "acme-invoicing-documents"
  key_name   = "application"
  visibility = "private"

  # cors_settings is a nested attribute, not a block, so it takes an object.
  cors_settings = {
    allowed_origins = ["https://${var.domain}"]
    allowed_methods = ["GET", "PUT"]
    allowed_headers = ["*"]
    max_age_seconds = 3600
  }
}

resource "laravel_cloud_storage_bucket_key" "renderer" {
  bucket_id  = laravel_cloud_storage_bucket.invoices.id
  name       = "pdf-renderer"
  permission = "read_only"
}

# ===========================================================================
# Production
# ===========================================================================

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.invoicing.id
  name           = "production"
  branch         = "main"

  php_version  = "8.4:1"
  node_version = "22"

  # Attaching the cache and database here is what injects their connection
  # details into the environment -- you do not repeat the credentials in
  # laravel_cloud_environment_variables.
  cache_id           = laravel_cloud_cache.production.id
  database_schema_id = laravel_cloud_database.production.id

  uses_push_to_deploy = true
  uses_octane         = true

  build_command = "composer install --no-dev --optimize-autoloader && npm ci && npm run build"

  color = "green"
}

resource "laravel_cloud_instance" "production_web" {
  environment_id = laravel_cloud_environment.production.id
  name           = "web"
  size           = "flex.c-2vcpu-1gb"
  uses_octane    = true

  # With scaling_type = "auto" the platform picks the replica count from these
  # thresholds. The API rejects min_replicas / max_replicas in this mode --
  # they only apply to scaling_type = "custom".
  scaling_type                        = "auto"
  scaling_cpu_threshold_percentage    = 70
  scaling_memory_threshold_percentage = 80
}

resource "laravel_cloud_domain" "production" {
  environment_id = laravel_cloud_environment.production.id
  name           = var.domain

  # pre_verification lets the certificate be issued before you cut traffic
  # over, so the switch itself is not a visible outage.
  verification_method = "pre_verification"
  www_redirect        = "www_to_root"
  cloudflare_strategy = "none"

  # Read dns_records off this resource to see exactly what to create at your
  # DNS provider -- see outputs.tf.
}

resource "laravel_cloud_environment_variables" "production" {
  environment_id = laravel_cloud_environment.production.id

  # This resource owns the environment's entire variable set: anything set in
  # the dashboard but missing here is removed on apply.
  variables = {
    APP_ENV   = "production"
    APP_DEBUG = "false"
    APP_URL   = "https://${var.domain}"

    LOG_CHANNEL = "stderr"
    LOG_LEVEL   = "warning"

    CACHE_STORE           = "redis"
    SESSION_DRIVER        = "redis"
    SESSION_SECURE_COOKIE = "true"
    QUEUE_CONNECTION      = "redis"

    FILESYSTEM_DISK       = "s3"
    AWS_BUCKET            = laravel_cloud_storage_bucket.invoices.name
    AWS_ENDPOINT          = laravel_cloud_storage_bucket.invoices.endpoint
    AWS_ACCESS_KEY_ID     = laravel_cloud_storage_bucket_key.renderer.access_key_id
    AWS_SECRET_ACCESS_KEY = laravel_cloud_storage_bucket_key.renderer.access_key_secret
  }
}

# ===========================================================================
# Staging
# ===========================================================================

resource "laravel_cloud_environment" "staging" {
  application_id = laravel_cloud_application.invoicing.id
  name           = "staging"
  branch         = "develop"

  php_version  = "8.4:1"
  node_version = "22"

  database_schema_id = laravel_cloud_database.staging.id

  uses_push_to_deploy = true

  # Let staging fall asleep quickly -- it spends most of the week idle.
  sleep_timeout = 5

  color = "orange"
}

resource "laravel_cloud_instance" "staging_web" {
  environment_id = laravel_cloud_environment.staging.id
  name           = "web"
  size           = "flex.c-1vcpu-256mb"

  scaling_type = "custom"
  min_replicas = 1
  max_replicas = 1
}

resource "laravel_cloud_environment_variables" "staging" {
  environment_id = laravel_cloud_environment.staging.id

  variables = {
    APP_ENV   = "staging"
    APP_DEBUG = "true"

    LOG_CHANNEL = "stderr"
    LOG_LEVEL   = "debug"

    # No cache attached to staging, so everything falls back to the database.
    CACHE_STORE      = "database"
    SESSION_DRIVER   = "database"
    QUEUE_CONNECTION = "database"

    # Never let a staging deploy mail a real customer.
    MAIL_MAILER = "log"
  }
}
