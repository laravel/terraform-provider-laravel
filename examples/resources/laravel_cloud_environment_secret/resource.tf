resource "laravel_cloud_secret" "stripe" {
  key   = "STRIPE_SECRET"
  value = var.stripe_secret
}

resource "laravel_cloud_environment_secret" "stripe" {
  environment_id = laravel_cloud_environment.example.id
  secret_id      = laravel_cloud_secret.stripe.id
}
