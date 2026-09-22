output "api_url" {
  value = "https://${laravel_cloud_domain.api.name}"
}

output "api_dns_records" {
  description = "DNS records required for the API domain to verify."
  value       = laravel_cloud_domain.api.dns_records
}

output "reverb_hostname" {
  description = "Host clients connect to for websocket events."
  value       = laravel_cloud_websocket_server.reverb.hostname
}

output "reverb_app_id" {
  value = laravel_cloud_websocket_application.ledger.app_id
}

# The queue instance reports its raw status JSON, which is the quickest way to
# see whether jobs are backing up.
output "queue_status" {
  value = laravel_cloud_instance.queue.queue_status
}
