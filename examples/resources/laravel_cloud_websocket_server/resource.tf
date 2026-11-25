# A Laravel Reverb server. Laravel Cloud creates an application named
# "production" on it; add more with laravel_cloud_websocket_application.
resource "laravel_cloud_websocket_server" "example" {
  name   = "my-app-reverb"
  type   = "reverb"
  region = "us-east-2"

  # One of 100, 200, 500, 2000, 5000 or 10000, as the plan allows (see the
  # laravel_cloud_websocket_server_types data source).
  max_connections = 500
}

output "reverb_host" {
  value = laravel_cloud_websocket_server.example.hostname
}
