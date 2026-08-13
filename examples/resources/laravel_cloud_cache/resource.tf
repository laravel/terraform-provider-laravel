resource "laravel_cloud_cache" "example" {
  name                 = "my-valkey"
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = true
  is_public            = false
}
