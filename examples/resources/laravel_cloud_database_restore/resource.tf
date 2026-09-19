resource "laravel_cloud_database_cluster" "example" {
  name    = "my-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"

  # `config` is an opaque JSON document whose shape depends on the database
  # type. Every key the API returns must be present, with exactly the value it
  # stores, or the next plan shows a permanent diff. Discover the schema and
  # the valid `size` enum with `GET /databases/types`.
  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 7
    suspend_seconds          = 0
  })
}

resource "laravel_cloud_database_snapshot" "example" {
  cluster_id = laravel_cloud_database_cluster.example.id
  name       = "nightly"
}

resource "laravel_cloud_database_restore" "example" {
  database_cluster_id  = laravel_cloud_database_cluster.example.id
  name                 = "restored-db"
  database_snapshot_id = laravel_cloud_database_snapshot.example.id
}
