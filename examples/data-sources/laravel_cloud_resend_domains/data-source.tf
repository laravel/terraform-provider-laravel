data "laravel_cloud_resend_domains" "all" {}

output "verified_sending_domains" {
  value = [for d in data.laravel_cloud_resend_domains.all.domains : d.name if d.status == "verified"]
}
