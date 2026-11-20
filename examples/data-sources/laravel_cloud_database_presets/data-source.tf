data "laravel_cloud_database_presets" "mysql" {
  type   = "laravel_mysql"
  region = "us-east-2"
}

# The settings of the "prod" preset, such as size and storage.
locals {
  prod_preset = jsondecode(one([
    for p in data.laravel_cloud_database_presets.mysql.presets : p.config if p.key == "prod"
  ]))
}
