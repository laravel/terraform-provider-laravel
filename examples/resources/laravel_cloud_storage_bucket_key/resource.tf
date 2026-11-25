resource "laravel_cloud_storage_bucket" "uploads" {
  name     = "my-app-uploads"
  key_name = "app"
}

# A read-write key the environment uses to reach the bucket.
resource "laravel_cloud_storage_bucket_key" "app" {
  bucket_id  = laravel_cloud_storage_bucket.uploads.id
  name       = "my-app"
  permission = "read_write"
}

# A read-only key for a backup job that copies the bucket elsewhere.
resource "laravel_cloud_storage_bucket_key" "backup" {
  bucket_id  = laravel_cloud_storage_bucket.uploads.id
  name       = "backup"
  permission = "read_only"
}

resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

# Attach the bucket to the environment through the app key, as its default
# disk.
resource "laravel_cloud_environment" "example" {
  application_id = laravel_cloud_application.example.id
  name           = "production"
  branch         = "main"
  php_version    = "8.4:1"

  filesystem_keys = [{
    key_id          = laravel_cloud_storage_bucket_key.app.id
    disk            = "s3"
    is_default_disk = true
  }]
}

# The secret is returned only when the key is created.
output "backup_access_key_id" {
  value = laravel_cloud_storage_bucket_key.backup.access_key_id
}

output "backup_access_key_secret" {
  value     = laravel_cloud_storage_bucket_key.backup.access_key_secret
  sensitive = true
}
