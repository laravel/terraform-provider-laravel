# ===========================================================================
# Acme Ledger: a queue-heavy JSON API with a scheduler and a Reverb
# websocket tier.
#
# The web instance stays small because it does almost nothing but accept
# requests and push jobs. The work happens on managed queues, and
# clients watch it land over websockets instead of polling.
# ===========================================================================

resource "laravel_cloud_application" "ledger" {
  name       = "acme-ledger"
  repository = var.repository
  region     = var.region

  # Required by the API from March 9, 2026.
  source_control_provider_type = "github"
}

resource "laravel_cloud_environment" "production" {
  application_id = laravel_cloud_application.ledger.id
  name           = "production"
  branch         = "main"

  php_version  = "8.4:1"
  node_version = "22"

  cache_id = laravel_cloud_cache.cache.id

  uses_push_to_deploy = true

  # Octane keeps the framework booted between requests, which is most of the
  # latency on a JSON API this thin.
  uses_octane = true

  deploy_command = "php artisan migrate --force && php artisan config:cache && php artisan event:cache"

  # An API request that has not answered in 30 seconds is not going to.
  timeout = 30
}

# Valkey cache for the cache store and sessions. Jobs go to the managed
# queues below, not here.
resource "laravel_cloud_cache" "cache" {
  name                 = "acme-ledger-cache"
  type                 = "laravel_valkey"
  region               = var.region
  size                 = "valkey-flex-1gb"
  auto_upgrade_enabled = true
  is_public            = false
}

resource "laravel_cloud_domain" "api" {
  environment_id = laravel_cloud_environment.production.id
  name           = var.domain

  verification_method = "pre_verification"
}

# --- Web tier --------------------------------------------------------------

# The app instance Laravel Cloud creates with the environment; type = "app"
# takes it over.
resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.production.id
  name           = "web"
  type           = "app"
  size           = "flex-1gb"

  # Octane is turned on with the environment's uses_octane, above.
  scaling_type                     = "auto"
  scaling_cpu_threshold_percentage = 65
}

# --- Queue tier ------------------------------------------------------------

# Managed queues: each runs its own workers, which scale on queue depth and
# to zero when the queue drains, so an idle night costs nothing. Laravel
# Cloud sets QUEUE_CONNECTION=cloud once the first queue is deployed, and a
# job has 180 seconds before the queue hands it to another worker.

# Jobs dispatched without a queue name land here.
resource "laravel_cloud_instance" "queue_default" {
  environment_id = laravel_cloud_environment.production.id
  name           = "ledger-default"
  type           = "managed_queue"
  size           = "mq.flex.512mb"
  is_default     = true

  scaling_type = "custom"
  min_replicas = 0
  max_replicas = 10
}

# Payment jobs, dispatched with ->onQueue('ledger-high'), get their own
# workers so a backlog of reports cannot hold them up.
resource "laravel_cloud_instance" "queue_high" {
  environment_id = laravel_cloud_environment.production.id
  name           = "ledger-high"
  type           = "managed_queue"
  size           = "mq.flex.256mb"

  scaling_type = "custom"
  min_replicas = 0
  max_replicas = 5

  depends_on = [laravel_cloud_instance.queue_default]
}

# --- Scheduler and daemons -------------------------------------------------

# One dedicated instance runs the scheduler. Running it on the web tier means
# every replica fires the same command.
resource "laravel_cloud_instance" "scheduler" {
  environment_id = laravel_cloud_environment.production.id
  name           = "scheduler"
  size           = "flex-512mb"
  uses_scheduler = true

  scaling_type = "custom"
  min_replicas = 1
  max_replicas = 1
}

# A long-lived daemon that is not a queue worker: it streams the bank feed and
# writes transactions as they arrive. It runs on the single scheduler
# replica, so there is exactly one stream. `command` is required when type is
# "custom".
resource "laravel_cloud_background_process" "bank_feed" {
  instance_id = laravel_cloud_instance.scheduler.id
  type        = "custom"
  processes   = 1
  command     = "php artisan ledger:stream-bank-feed --timeout=0"
}

# --- Websocket tier --------------------------------------------------------

resource "laravel_cloud_websocket_server" "reverb" {
  name   = "acme-ledger-reverb"
  type   = "reverb"
  region = var.region

  # Valid values: 100, 200, 500, 2000, 5000, 10000.
  max_connections = 2000
}

resource "laravel_cloud_websocket_application" "ledger" {
  server_id = laravel_cloud_websocket_server.reverb.id
  name      = "ledger-events"

  # allowed_origins is computed as well as optional: dropping the attribute
  # leaves the existing origins alone. Assign [] to actually clear them.
  allowed_origins = [
    "https://${var.domain}",
    "https://app.example.com",
  ]

  ping_interval    = 30
  activity_timeout = 60
}

# --- Wiring ----------------------------------------------------------------

resource "laravel_cloud_environment_variables" "production" {
  environment_id = laravel_cloud_environment.production.id

  # This resource sets the variables listed here and deletes a variable when
  # it is removed from this map. Variables set in the dashboard and never
  # listed here are left alone, but a key listed here overwrites the
  # dashboard's value on every apply.
  variables = {
    APP_ENV   = "production"
    APP_DEBUG = "false"
    APP_URL   = "https://${var.domain}"

    LOG_CHANNEL = "stderr"
    LOG_LEVEL   = "warning"

    CACHE_STORE    = "redis"
    SESSION_DRIVER = "redis"

    # Reverb credentials come off the websocket application rather than being
    # copied in by hand.
    BROADCAST_CONNECTION = "reverb"
    REVERB_APP_ID        = laravel_cloud_websocket_application.ledger.app_id
    REVERB_APP_KEY       = laravel_cloud_websocket_application.ledger.key
    REVERB_APP_SECRET    = laravel_cloud_websocket_application.ledger.secret
    REVERB_HOST          = laravel_cloud_websocket_server.reverb.hostname
    REVERB_PORT          = "443"
    REVERB_SCHEME        = "https"
  }
}
