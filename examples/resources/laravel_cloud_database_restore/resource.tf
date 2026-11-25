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

resource "laravel_cloud_database_snapshot" "before_migration" {
  cluster_id = laravel_cloud_database_cluster.example.id
  name       = "before-2026-11-migration"
}

# Restores the snapshot into a new cluster; the source cluster is unchanged.
resource "laravel_cloud_database_restore" "from_snapshot" {
  database_cluster_id  = laravel_cloud_database_cluster.example.id
  name                 = "my-app-db-restored"
  database_snapshot_id = laravel_cloud_database_snapshot.before_migration.id

  # The restored cluster holds copies of the source cluster's databases.
  force_destroy = true
}

# Or restore the cluster as it was at a point in time:
#
# resource "laravel_cloud_database_restore" "point_in_time" {
#   database_cluster_id = laravel_cloud_database_cluster.example.id
#   name                = "my-app-db-pitr"
#   restore_time        = "2026-11-20T09:30:00Z"
# }
