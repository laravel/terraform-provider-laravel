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

# Laravel Cloud creates an app instance with every environment. type = "app"
# takes it over instead of creating another, and destroy leaves it to the
# environment.
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.example.id
  name           = "App"
  type           = "app"
  size           = "flex-1gb"

  scaling_type                     = "custom"
  min_replicas                     = 1
  max_replicas                     = 3
  scaling_cpu_threshold_percentage = 70

  uses_octane         = true
  hibernation_timeout = 0 # never hibernate
}

# A managed queue for the jobs the app dispatches without naming a queue.
# Its workers scale on queue depth between min_replicas and max_replicas, and
# to zero when the queue is empty. Sizes are the current-generation mq.* sizes
# the laravel_cloud_instance_sizes data source lists with instance_class
# "managed_queue". Laravel Cloud sets their visibility timeout (180 seconds)
# and shutdown timeout (90 seconds). The other queues are created after it,
# because Laravel Cloud makes an environment's first queue its default.
resource "laravel_cloud_instance" "default_queue" {
  environment_id = laravel_cloud_environment.example.id
  name           = "default"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  is_default     = true

  scaling_type = "custom"
  min_replicas = 0
  max_replicas = 5
}

# A queue with larger workers for heavy jobs, which the app dispatches with
# ->onQueue('exports').
resource "laravel_cloud_instance" "exports_queue" {
  environment_id = laravel_cloud_environment.example.id
  name           = "exports"
  type           = "managed_queue"
  size           = "mq.flex.2gb"

  scaling_type = "custom"
  min_replicas = 0
  max_replicas = 2

  depends_on = [laravel_cloud_instance.default_queue]
}

# A FIFO queue delivers jobs once and in order. Laravel Cloud names it
# "orders.fifo"; dispatch to it with ->onQueue('orders.fifo').
resource "laravel_cloud_instance" "orders_queue" {
  environment_id = laravel_cloud_environment.example.id
  name           = "orders"
  type           = "managed_queue"
  size           = "mq.flex.512mb"
  fifo           = true

  scaling_type = "custom"
  min_replicas = 0
  max_replicas = 1

  depends_on = [laravel_cloud_instance.default_queue]
}
