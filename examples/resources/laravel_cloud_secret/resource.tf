variable "stripe_secret" {
  type      = string
  sensitive = true
}

# Terraform 1.11 or later: the value is sent but never stored in state.
# Bump value_wo_version to send a new value.
resource "laravel_cloud_secret" "stripe" {
  key              = "STRIPE_SECRET"
  value_wo         = var.stripe_secret
  value_wo_version = 1
  notes            = "Stripe live key"
}

# Earlier Terraform versions: the value is kept in state, marked sensitive.
resource "laravel_cloud_secret" "mailgun" {
  key   = "MAILGUN_SECRET"
  value = var.mailgun_secret
}

variable "mailgun_secret" {
  type      = string
  sensitive = true
}
