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
    is_public                = false
    uses_scheduled_snapshots = true
    retention_days           = 7
  })
}

resource "laravel_cloud_database" "example" {
  cluster_id = laravel_cloud_database_cluster.example.id
  name       = "my_app"
}
