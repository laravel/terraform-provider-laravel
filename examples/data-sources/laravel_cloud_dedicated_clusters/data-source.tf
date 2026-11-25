data "laravel_cloud_dedicated_clusters" "all" {}

# Filters match as substrings.
data "laravel_cloud_dedicated_clusters" "apps_us_east_2" {
  region = "us-east-2"
  type   = "applications"
  status = "active"
}
