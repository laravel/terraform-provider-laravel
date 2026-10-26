variable "stripe_secret" {
  type      = string
  sensitive = true
}

resource "laravel_cloud_secret" "stripe" {
  key   = "STRIPE_SECRET"
  value = var.stripe_secret
  notes = "Stripe live key"
}
