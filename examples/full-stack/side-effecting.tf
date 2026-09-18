# ===========================================================================
# Tier 2 -- side-effecting resources (4)
#
# These do more than CRUD and are OFF by default. Enable with:
#
#   terraform plan  -var enable_side_effecting=true     # validate schemas
#   terraform apply -var enable_side_effecting=true     # actually run them
#
# (run.sh: `SIDE_EFFECTING=1 ./run.sh apply`)
#
# Caveats when applying for real:
#  - deployment triggers a real build/deploy of the environment; it needs the
#    GitHub repo connected and the branch buildable, or it will fail/hang.
#  - command runs a real artisan command and needs a deployed environment to
#    run against.
#  - database_snapshot creates a real snapshot (slow, but harmless).
#  - database_restore restores from the snapshot above. The snapshot must be
#    finished before the restore call succeeds -- a single apply may race it.
#    Re-run apply once the snapshot reports available, or restore from a PITR
#    timestamp instead (set restore_time to a moment inside the cluster's live
#    retention window; the snapshot reference below is usually more reliable).
# ===========================================================================

resource "laravel_cloud_deployment" "deploy" {
  count          = var.enable_side_effecting ? 1 : 0
  environment_id = laravel_cloud_environment.env.id
}

resource "laravel_cloud_command" "command" {
  count          = var.enable_side_effecting ? 1 : 0
  environment_id = laravel_cloud_environment.env.id
  command        = "php artisan migrate --force"

  # Run the command only after a deploy exists to run it against.
  depends_on = [laravel_cloud_deployment.deploy]
}

resource "laravel_cloud_database_snapshot" "snap" {
  count      = var.enable_side_effecting && var.enable_database ? 1 : 0
  cluster_id = laravel_cloud_database_cluster.cluster[0].id
  name       = "${var.prefix}-snapshot"
}

resource "laravel_cloud_database_restore" "restore" {
  count                = var.enable_side_effecting && var.enable_database ? 1 : 0
  database_cluster_id  = laravel_cloud_database_cluster.cluster[0].id
  name                 = "${var.prefix}-restored"
  database_snapshot_id = laravel_cloud_database_snapshot.snap[0].id
}

output "deployment_status" {
  value = var.enable_side_effecting ? laravel_cloud_deployment.deploy[0].status : null
}

output "command_status" {
  value = var.enable_side_effecting ? laravel_cloud_command.command[0].status : null
}
