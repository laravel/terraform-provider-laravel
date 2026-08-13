# ------------------------------------------------------------------
# Locals
# ------------------------------------------------------------------

locals {
  app_count = 50
  app_base  = "quantum-burrito" # change me for a different vibe (e.g. "disco-llama", "yeeting-yak")
}

# ------------------------------------------------------------------
# Shared Database Cluster (one cluster, 50 schemas)
# ------------------------------------------------------------------



# ------------------------------------------------------------------
# 50 Applications
# ------------------------------------------------------------------

resource "laravel_cloud_application" "fleet" {
  count      = local.app_count
  name       = format("%s-%02d", local.app_base, count.index + 1)
  repository = "AlfonsoCampodonico/laravel-dev-starter"
  region     = "us-east-2"
}

# ------------------------------------------------------------------
# Production env per app
# ------------------------------------------------------------------

resource "laravel_cloud_environment" "production" {
  count          = local.app_count
  application_id = laravel_cloud_application.fleet[count.index].id
  name           = "production"
  branch         = "main"
  php_version    = "8.4:1"
  node_version   = "22"

  uses_push_to_deploy = true
  uses_octane         = true
  timeout             = 30

}
