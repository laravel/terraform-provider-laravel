resource "laravel_cloud_scheduled_command" "nightly_reports" {
  instance_id = laravel_cloud_instance.example.id
  name        = "Nightly reports"
  command     = "php artisan reports:send"
  schedule    = "0 3 * * *" # 03:00 UTC every day
}
