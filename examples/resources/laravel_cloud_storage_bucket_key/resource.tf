resource "laravel_cloud_storage_bucket" "example" {
  name     = "my-app-assets"
  key_name = "primary"
}

resource "laravel_cloud_storage_bucket_key" "example" {
  bucket_id  = laravel_cloud_storage_bucket.example.id
  name       = "deploy-key"
  permission = "read_write"
}
