# Keys are addressed through their bucket, so the import ID is composite.
# Note: the API never returns access_key_secret again after creation, so an
# imported key has a null secret in state.
terraform import laravel_cloud_storage_bucket_key.example "{bucket_id}:{key_id}"
