resource "laravel_cloud_websocket_server" "example" {
  name   = "my-reverb"
  type   = "reverb"
  region = "us-east-2"

  # Valid values: 100, 200, 500, 2000, 5000, 10000.
  max_connections = 5000
}
