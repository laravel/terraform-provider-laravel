# The API never returns a secret's value, so it is not imported; the first
# apply after an import writes the configured value.
terraform import laravel_cloud_secret.stripe "{secret_id}"
