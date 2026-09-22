# ===========================================================================
# Acme Ledger -- a queue-heavy JSON API with a scheduler and a Reverb
# websocket tier.
#
# The web instance stays small because it does almost nothing but accept
# requests and push jobs. The work happens on a managed queue instance, and
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

  cache_id = laravel_cloud_cache.queue.id

  uses_push_to_deploy = true
  uses_octane         = true

  deploy_command = "php artisan migrate --force && php artisan config:cache && php artisan event:cache"

  # An API request that has not answered in 30 seconds is not going to.
  timeout = 30
}

# Redis-compatible cache, used here as the queue backend as well as the cache.
resource "laravel_cloud_cache" "queue" {
  name                 = "acme-ledger-queue"
  type                 = "laravel_valkey"
  region               = var.region
  size                 = "valkey-flex-500mb"
  auto_upgrade_enabled = true
  is_public            = false

  # Jobs must not be evicted under memory pressure the way cache entries can.
  eviction_policy = "noeviction"
}

resource "laravel_cloud_domain" "api" {
  environment_id = laravel_cloud_environment.production.id
  name           = var.domain

  verification_method = "pre_verification"
}

# --- Web tier --------------------------------------------------------------

resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.production.id
  name           = "web"
  size           = "flex.c-1vcpu-512mb"

  # Octane keeps the framework booted between requests, which is most of the
  # latency on a JSON API this thin.
  uses_octane = true

  scaling_type                     = "auto"
  scaling_cpu_threshold_percentage = 65
}

# --- Queue tier ------------------------------------------------------------

# A managed_queue instance scales to zero when the queue drains, so an idle
# night costs nothing. min_replicas does not apply to it for that reason.
resource "laravel_cloud_instance" "queue" {
  environment_id = laravel_cloud_environment.production.id
  name           = "queue"
  type           = "managed_queue"
  size           = "flex.c-1vcpu-512mb"
  scaling_type   = "auto"

  # Give a job 5 minutes to finish before the queue hands it to someone else,
  # and 2 minutes to wind down when a replica is being taken away.
  visibility_timeout = 300
  shutdown_timeout   = 120

  # Follow the app to sleep -- there is nothing to process while it is down.
  sleep_with_app = true
}

# The default worker, reading the two queues in priority order.
resource "laravel_cloud_background_process" "worker" {
  instance_id = laravel_cloud_instance.queue.id
  type        = "worker"
  processes   = 4

  config = jsonencode({
    connection = "redis"
    queue      = "ledger-high,ledger-default"
    tries      = 3
    backoff    = 30
    timeout    = 120
    sleep      = 3
    rest       = 0
    force      = false
  })
}

# A long-lived daemon that is not a queue worker: it streams the bank feed and
# writes transactions as they arrive. `command` is required when type is
# "custom".
resource "laravel_cloud_background_process" "bank_feed" {
  instance_id = laravel_cloud_instance.queue.id
  type        = "custom"
  processes   = 1
  command     = "php artisan ledger:stream-bank-feed --timeout=0"
}

# --- Scheduler -------------------------------------------------------------

# One dedicated instance runs the scheduler. Running it on the web tier means
# every replica fires the same command.
resource "laravel_cloud_instance" "scheduler" {
  environment_id = laravel_cloud_environment.production.id
  name           = "scheduler"
  size           = "flex.c-1vcpu-256mb"
  uses_scheduler = true

  scaling_type = "custom"
  min_replicas = 1
  max_replicas = 1
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
  activity_timeout = 120
}

# --- Wiring ----------------------------------------------------------------

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

    CACHE_STORE      = "redis"
    QUEUE_CONNECTION = "redis"
    SESSION_DRIVER   = "redis"

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
