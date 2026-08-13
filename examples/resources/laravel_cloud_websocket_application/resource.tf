resource "laravel_cloud_websocket_server" "example" {
  name   = "my-reverb"
  type   = "reverb"
  region = "us-east-2"

  # Valid values: 100, 200, 500, 2000, 5000, 10000.
  max_connections = 5000
}

resource "laravel_cloud_websocket_application" "example" {
  server_id       = laravel_cloud_websocket_server.example.id
  name            = "chat"
  allowed_origins = ["https://example.com"]

  ping_interval    = 30
  activity_timeout = 60
}
