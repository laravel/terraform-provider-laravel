data "laravel_cloud_websocket_server_types" "all" {}

# The max_connections tiers the organization's plan includes for Reverb.
locals {
  reverb_tiers = [
    for tier in one([for t in data.laravel_cloud_websocket_server_types.all.types : t if t.type == "reverb"]).max_connections :
    tier.value if tier.available
  ]
}
