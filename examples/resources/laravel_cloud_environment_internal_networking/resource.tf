# Let only the API environment reach the backend over the private network.
resource "laravel_cloud_environment_internal_networking" "backend" {
  environment_id  = laravel_cloud_environment.backend.id
  access_level    = "specific"
  environment_ids = [laravel_cloud_environment.api.id]
}

# The API reaches the backend at this hostname.
output "backend_internal_hostname" {
  value = laravel_cloud_environment_internal_networking.backend.hostname
}
