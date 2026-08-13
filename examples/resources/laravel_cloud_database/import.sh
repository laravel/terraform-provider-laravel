# Databases are addressed through their cluster, so the import ID is composite.
terraform import laravel_cloud_database.example "{cluster_id}:{database_id}"
