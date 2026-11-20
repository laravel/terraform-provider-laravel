# Database users are addressed through their cluster, so the import ID is
# composite. An imported user has no password in state: the API returns it
# only when the user is created. The cluster's superuser cannot be imported.
terraform import laravel_cloud_database_user.example "{cluster_id}:{user_id}"
