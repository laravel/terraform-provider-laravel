data "laravel_cloud_edge_networks" "available" {}

# Filters match as substrings.
data "laravel_cloud_edge_networks" "mine" {
  domain = "example.com"
  status = "available"
}
