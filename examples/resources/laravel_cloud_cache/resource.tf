resource "laravel_cloud_cache" "example" {
  name                 = "my-valkey"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false

  # Scale to zero after 5 idle minutes. The API leaves it off unless set.
  uses_hibernation    = true
  hibernation_timeout = 5
}

# An AWS ElastiCache cache on one of the organization's dedicated (Private
# Cloud) clusters. Pin the cluster: without cluster_id the API picks one of the
# region's clusters in no defined order, and no endpoint reports which.
data "laravel_cloud_dedicated_clusters" "elasticache" {
  region = "us-east-2"
  type   = "elasticache-clusters"
  status = "active"
}

resource "laravel_cloud_cache" "elasticache" {
  name                 = "my-elasticache"
  type                 = "aws_elasticache_valkey"
  region               = "us-east-2"
  size                 = "elasticache.cache.t4g.micro"
  auto_upgrade_enabled = false # not supported for ElastiCache
  cluster_id           = data.laravel_cloud_dedicated_clusters.elasticache.clusters[0].id
  # is_public is left out: ElastiCache caches are always private.
}
