# Rules apply to a dedicated edge network.
data "laravel_cloud_edge_networks" "mine" {
  domain = "example.com"
  status = "available"
}

locals {
  edge_network_id = data.laravel_cloud_edge_networks.mine.edge_networks[0].id
}

# Block WordPress login probes.
resource "laravel_cloud_edge_network_rule" "block_wp" {
  edge_network_id = local.edge_network_id
  ruleset         = "firewall"
  name            = "Block WordPress probes"
  description     = "This is not a WordPress site"
  settings = jsonencode({
    action = "block"
    criteria_groups = [
      { criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/wp-login.php" }] },
      { criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/xmlrpc.php" }] },
    ]
  })
}

# Challenge clients that post to /login more than 10 times a minute, for 10
# minutes. It is the first rule of its ruleset.
resource "laravel_cloud_edge_network_rule" "login_rate_limit" {
  edge_network_id = local.edge_network_id
  ruleset         = "rate_limit"
  name            = "Throttle logins"
  settings = jsonencode({
    requests_per_period = 10
    period              = 60
    action              = "managed_challenge"
    action_duration     = 600
    throttles           = false
    criteria_groups = [{
      criterias = [
        { criteria = "value", field = "uri_path", operator = "equals", value = "/login" },
        { criteria = "value", field = "request_method", operator = "equals", value = "POST" },
      ]
    }]
  })
  position = { index = 1 }
}
