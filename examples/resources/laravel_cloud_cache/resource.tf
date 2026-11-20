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
