data "laravel_cloud_instance_sizes" "available" {}

# Sizes for app and service instances, and the current-generation (mq.*)
# sizes for managed queues.
locals {
  instance_sizes      = [for s in data.laravel_cloud_instance_sizes.available.sizes : s.name if s.instance_class == "general"]
  managed_queue_sizes = [for s in data.laravel_cloud_instance_sizes.available.sizes : s.name if s.instance_class == "managed_queue" && startswith(s.name, "mq.")]
}
