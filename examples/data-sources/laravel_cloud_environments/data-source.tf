data "laravel_cloud_applications" "shop" {
  slug = "shop"
}

# Filters match as substrings, so pick the exact match out of the list.
data "laravel_cloud_environments" "staging" {
  application_id = one([for a in data.laravel_cloud_applications.shop.applications : a.id if a.slug == "shop"])
  name           = "staging"
}

# Attach a domain to an environment managed outside this configuration.
resource "laravel_cloud_domain" "staging" {
  environment_id = one([for e in data.laravel_cloud_environments.staging.environments : e.id if e.name == "staging"])
  name           = "staging.example.com"
}
