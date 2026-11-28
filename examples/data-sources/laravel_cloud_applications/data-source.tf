data "laravel_cloud_applications" "all" {}

# Filters match as substrings, so pick the exact match out of the list.
data "laravel_cloud_applications" "shop" {
  name   = "shop"
  region = "us-east-2"
}

locals {
  shop = one([for a in data.laravel_cloud_applications.shop.applications : a if a.name == "shop"])
}

output "shop_default_environment_id" {
  value = local.shop.default_environment_id
}
