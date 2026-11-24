resource "laravel_cloud_database_cluster" "example" {
  name    = "my-app-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"

  # The settings of the database type, as JSON. size is one of the sizes the
  # laravel_cloud_database_types data source lists for the type.
  config = jsonencode({
    size                     = "mysql-flex-1gb"
    storage                  = 10 # GiB
    storage_autoscale_max_gb = 50 # grow storage on its own, up to 50 GiB
    is_public                = false
    uses_scheduled_snapshots = true
    retention_days           = 7
    maintenance_window       = "sun:06:00-sun:06:30" # UTC
  })

  # The cluster starts with a default database; let destroy remove it.
  force_destroy = true
}

# A Neon Serverless Postgres cluster, which scales to zero when idle.
resource "laravel_cloud_database_cluster" "neon" {
  name    = "my-app-pg"
  type    = "neon_serverless_postgres"
  version = "18"
  region  = "us-east-2"

  # Compute units scale between cu_min and cu_max; the database suspends after
  # suspend_seconds without connections.
  config = jsonencode({
    cu_min          = 0.25
    cu_max          = 1
    suspend_seconds = 300
    retention_days  = 1
  })
}
