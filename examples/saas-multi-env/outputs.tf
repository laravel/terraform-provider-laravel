output "production_url" {
  description = "Where production will serve once the domain verifies."
  value       = "https://${laravel_cloud_domain.production.name}"
}

# Create these at your DNS provider. Until they exist, hostname_status and
# ssl_status stay pending and the domain serves nothing.
output "production_dns_records" {
  description = "DNS records required for the production domain to verify."
  value       = laravel_cloud_domain.production.dns_records
}

output "production_domain_action_required" {
  description = "What is still outstanding on the production domain, or null when it has verified."
  value       = laravel_cloud_domain.production.action_required
}

output "staging_status" {
  value = laravel_cloud_environment.staging.status
}

output "database_cluster_status" {
  value = laravel_cloud_database_cluster.primary.status
}

output "invoice_bucket_endpoint" {
  value = laravel_cloud_storage_bucket.invoices.endpoint
}
