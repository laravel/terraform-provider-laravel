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

# A worker instance, so background processes do not compete with web traffic.
# For queued jobs, prefer a managed queue (laravel_cloud_instance with
# type = "managed_queue"), which runs and scales its own workers; background
# processes suit other queue drivers and long-running daemons.
resource "laravel_cloud_instance" "worker" {
  environment_id = laravel_cloud_environment.example.id
  name           = "worker"
  size           = "flex-1gb"
  scaling_type   = "custom"
  min_replicas   = 1
  max_replicas   = 2
}

# Two queue:work processes per replica on the database queue driver. Laravel
# Cloud generates the command from config.
resource "laravel_cloud_background_process" "worker" {
  instance_id = laravel_cloud_instance.worker.id
  type        = "worker"
  processes   = 2

  config = jsonencode({
    connection = "database"
    queue      = "default"
    tries      = 3
    backoff    = 10
    timeout    = 60
  })
}

# A long-running daemon that is not a queue worker.
resource "laravel_cloud_background_process" "pulse" {
  instance_id = laravel_cloud_instance.worker.id
  type        = "custom"
  processes   = 1
  command     = "php artisan pulse:check"
}
