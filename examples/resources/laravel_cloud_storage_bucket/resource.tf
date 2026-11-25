# A private bucket for user uploads, with its first access key.
resource "laravel_cloud_storage_bucket" "uploads" {
  name     = "my-app-uploads"
  key_name = "app"
}

# A public bucket for assets the browser loads directly.
resource "laravel_cloud_storage_bucket" "assets" {
  name          = "my-app-assets"
  key_name      = "deploy"
  visibility    = "public"
  location_hint = "enam"

  cors_settings = {
    allowed_origins = ["https://example.com"]
    allowed_methods = ["GET", "HEAD"]
    allowed_headers = ["*"]
    max_age_seconds = 3600
  }
}

output "assets_url" {
  value = laravel_cloud_storage_bucket.assets.url
}
