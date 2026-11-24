resource "laravel_cloud_websocket_server" "example" {
  name            = "my-app-reverb"
  type            = "reverb"
  region          = "us-east-2"
  max_connections = 500
}

resource "laravel_cloud_websocket_application" "chat" {
  server_id       = laravel_cloud_websocket_server.example.id
  name            = "chat"
  allowed_origins = ["https://example.com"]

  ping_interval    = 30
  activity_timeout = 60
}

resource "laravel_cloud_application" "example" {
  name                         = "my-app"
  repository                   = "my-org/my-app"
  region                       = "us-east-2"
  source_control_provider_type = "github"
}

# Attach the application to the environment, which sets its REVERB_*
# variables.
resource "laravel_cloud_environment" "example" {
  application_id           = laravel_cloud_application.example.id
  name                     = "production"
  branch                   = "main"
  php_version              = "8.4:1"
  websocket_application_id = laravel_cloud_websocket_application.chat.id
}
