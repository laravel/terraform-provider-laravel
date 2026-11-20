data "laravel_cloud_edge_networks" "mine" {
  domain = "example.com"
}

# Block the WordPress login probes.
resource "laravel_cloud_edge_network_rule" "block_wp" {
  edge_network_id = data.laravel_cloud_edge_networks.mine.edge_networks[0].id
  ruleset         = "firewall"
  name            = "Block WordPress probes"
  settings = jsonencode({
    action = "block"
    criteria_groups = [
      { criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/wp-login.php" }] },
      { criterias = [{ criteria = "value", field = "uri_path", operator = "equals", value = "/xmlrpc.php" }] },
    ]
  })
}

# Rate limit logins, evaluated before the firewall rule above.
resource "laravel_cloud_edge_network_rule" "login_rate_limit" {
  edge_network_id = data.laravel_cloud_edge_networks.mine.edge_networks[0].id
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
