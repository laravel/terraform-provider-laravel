# A Laravel Valkey cache that scales to zero after 5 idle minutes.
resource "laravel_cloud_cache" "example" {
  name                 = "my-app-cache"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false

  uses_hibernation    = true
  hibernation_timeout = 5
}

# A cache that must never drop keys under memory pressure, such as one that
# holds locks or rate limiter counters.
resource "laravel_cloud_cache" "locks" {
  name                 = "my-app-locks"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-pro.1gb"
  auto_upgrade_enabled = true
  is_public            = false
  eviction_policy      = "noeviction"
}

# An AWS ElastiCache cache on one of the organization's dedicated (Private
# Cloud) clusters. Pin the cluster: without cluster_id the API picks one of the
# region's clusters in no defined order.
data "laravel_cloud_dedicated_clusters" "elasticache" {
  region = "us-east-2"
  type   = "elasticache-clusters"
  status = "active"
}

resource "laravel_cloud_cache" "elasticache" {
  name                 = "my-app-elasticache"
  type                 = "aws_elasticache_valkey"
  region               = "us-east-2"
  size                 = "elasticache.cache.t4g.micro"
  auto_upgrade_enabled = false # not supported for ElastiCache
  cluster_id           = data.laravel_cloud_dedicated_clusters.elasticache.clusters[0].id
  # is_public is left out: ElastiCache caches are always private.
}
