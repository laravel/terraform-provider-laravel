# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Deprecated

- `laravel_cloud_environment.database_schema_id` is renamed to `database_id`,
  which takes the `id` of a `laravel_cloud_database`. The old name still works
  and warns; setting both is an error. Renaming it in a configuration shows as
  an update, but applying that plan only rewrites state and leaves the
  database attached. If `database_schema_id` is listed in
  `lifecycle.ignore_changes`, remove it from there when renaming; a plan that
  carries both attributes is rejected.

### Fixed

- The first plan after importing a `laravel_cloud_application` or
  `laravel_cloud_environment` no longer proposes adding `repository` or
  `php_version` when they already match the platform. Neither was read back:
  the API reports the repository as an object and the PHP version only as its
  major version, so an import left both null. An import now seeds both from
  the API -- `php_version` derived from `php_major_version` -- and a refresh
  still never rewrites either. `php_version` is now computed, so an imported
  value does not show as a change when the configuration leaves it unset.
- `cluster_id` and `source_control_provider_type` are still not reported by
  the API, so they still show as added on the first plan after an import. That
  plan now carries a warning explaining that, when nothing else changes,
  applying it only records the value and leaves the resource as it is.
- The first plan after importing a `laravel_cloud_database_cluster` no longer
  proposes changing `config`, and applying it no longer sends a config update
  to the cluster. An import records the API's full effective config, defaults
  the configuration never sets included, and the plan proposed removing them.
  `config` is now compared by content: when every configured key already has
  the same value, nothing is planned, and an update that leaves `config` alone
  sends nothing to the API.
- Planning an in-place update of a `laravel_cloud_database_cluster` -- which
  every plan after an import is, since `version` and `force_destroy` are never
  reported -- no longer prints "this attribute value will no longer be marked
  as sensitive" above `connection_details.password`. The whole object was
  planned as unknown, which cannot carry the nested sensitive mark. The
  recorded details, `status` and `created_at` are now kept unless `config`
  changes, and a config change plans each connection detail as unknown while
  keeping the password sensitive.

## [1.0.2] - 2026-09-19

### Fixed

- Importing a `laravel_cloud_environment`, `laravel_cloud_database_snapshot`,
  `laravel_cloud_command` or `laravel_cloud_deployment` no longer destroys the
  resource on the next plan. Each is imported by its own id and each carries a
  required parent attribute that forces replacement, but none of them read the
  parent back from the API, so the parent stayed null and Terraform proposed
  replacing what had just been imported -- deleting a live environment or a
  backup snapshot, or re-running a command. The parent is now requested with
  `?include=` and read from the relationships block. An imported environment
  also recovers its `branch`, which the API reports as a relationship rather
  than an attribute.
- Importing a `laravel_cloud_storage_bucket` no longer destroys the bucket and
  its contents. `key_name` and `key_permission` describe the initial access
  key and are never reported back, so after an import they were null while the
  configuration (and `key_permission`'s default) supplied a value, which the
  unconditional force-replacement read as a change. The same applied to
  `laravel_cloud_database_cluster.version` -- the argument this release
  recommends -- and to `cluster_id` on the cluster, application and
  environment resources. These attributes still force replacement whenever
  they genuinely change; only a missing prior value is exempt.
- Updating a `laravel_cloud_application` no longer clears its generated
  `slug`, and updating a `laravel_cloud_instance` no longer resets
  `sleep_with_app`, `visibility_timeout` and `shutdown_timeout` to zero
  values. An optional attribute the configuration leaves unset is unknown
  during an update, and the update request sent that unknown as an empty
  string or zero. The response was then written to state, so the change was
  invisible on later plans.
- List data sources return an empty list rather than a null one when there is
  nothing to report. `laravel_cloud_dedicated_clusters`,
  `laravel_cloud_edge_networks`, `laravel_cloud_regions`,
  `laravel_cloud_instance_sizes` and `laravel_cloud_cache_types` produced null,
  so `length()` and `for_each` over an empty result failed with "argument must
  not be null".
- An unknown `token` in the provider block now reports that it is unknown,
  instead of reporting the token as missing and discarding one supplied
  through `LARAVEL_CLOUD_API_TOKEN`.

### Changed

- The database examples and schema descriptions now use the current type
  identifiers. `laravel_mysql_84` and the other identifiers that bake the
  engine version into the type are retired -- the API reports them with an
  empty `versions` list -- so the examples paired the deprecated form with no
  `version` argument. They now use `type = "laravel_mysql"` with
  `version = "8.4"`, which is what new clusters should use, and the `type`
  descriptions on `laravel_cloud_database_cluster` and
  `laravel_cloud_database_types` name current identifiers and explain how to
  tell the two apart. Retired identifiers are still accepted; nothing needs
  changing in existing configurations.


## [1.0.1] - 2026-09-19

### Fixed

- The release's `SHA256SUMS` now covers the Terraform Registry protocol
  manifest. The Registry requires a checksum for every asset attached to a
  release, and `release.extra_files` only uploaded the manifest without
  listing it in the checksums, so ingest of v1.0.0 failed with "Could not
  find all required assets for this release yet: missing SHA256 checksum for
  terraform-provider-laravel_1.0.0_manifest.json". v1.0.0 was published on
  GitHub but never ingested, so 1.0.1 is the first version installable from
  the Registry.


## [1.0.0] - 2026-09-19

Initial public release of the Terraform provider for Laravel Cloud. The
provider's mapping onto the Laravel Cloud API was verified against the
published OpenAPI spec, and most behaviour was reproduced against a live API.

### Added

- Resources: `application`, `environment`, `instance`, `domain`,
  `database_cluster`, `database`, `database_snapshot`, `database_restore`,
  `cache`, `storage_bucket`, `storage_bucket_key`, `background_process`,
  `environment_variables`, `deployment`, `command`, `websocket_server`,
  `websocket_application`.
- Data sources: `organization`, `dedicated_clusters`, `instance_sizes`,
  `database_types`, `cache_types`, `regions`, `ip_addresses`, `edge_networks`.
- `laravel_cloud_environment.vanity_domain` is writable, through the API's
  dedicated endpoint.
- `laravel_cloud_database_cluster` supports `version` and `force_destroy`.
- `laravel_cloud_database_types` reports `versions`, needed to supply the
  cluster `version` argument.
- `laravel_cloud_instance_sizes` reports managed-queue sizes, and `cpu_count`
  is a number because managed-queue sizes express fractional vCPUs.
- `laravel_cloud_domain` reports `dns_records`, `stage`, `action_required` and
  `last_verified_at`, so a domain setup can be completed from Terraform.
- `source_control_provider_type` accepts `gitlab_self_hosted`.
- The `LARAVEL_CLOUD_BASE_URL` environment variable points the provider at an
  alternate host.

### Behaviour worth knowing

- `laravel_cloud_instance.polling_interval` is read-only. The API accepts it in
  neither the create nor the update request, so setting it has no effect.
- `laravel_cloud_instance.min_replicas` and `max_replicas` are rejected during
  validation alongside `scaling_type = "auto"`. The API documents them as
  "rejected when used with auto" and 422s such a request, so the error is
  raised at plan time rather than apply time.
- `laravel_cloud_domain`: changing `www_redirect`, `wildcard_enabled`,
  `allow_downtime` or `cloudflare_strategy` forces replacement, because the
  update endpoint accepts `verification_method` and nothing else. Replacement
  is destroy-before-create, so **the domain stops serving until it
  re-verifies**. Consider `lifecycle { prevent_destroy = true }` on production
  domains.
- `laravel_cloud_database_cluster` cannot be destroyed while it contains
  databases unless `force_destroy` is set, because the API refuses to delete a
  cluster with databases attached and a cluster always carries at least the one
  created with it. Setting `force_destroy = true` deletes every database in the
  cluster, including any this configuration does not manage.
- `laravel_cloud_websocket_application.allowed_origins` is optional and
  computed, because the API always returns a list there. Removing the attribute
  from a configuration does not clear the origins; assign
  `allowed_origins = []` instead.

### Known limitations

- The environment attributes `color`, `timeout`, `sleep_timeout`,
  `shutdown_timeout`, `uses_purge_edge_cache_on_deploy` and `cache_strategy`
  are accepted by the update endpoint but never returned, so they cannot be
  refreshed. Changing one outside Terraform is not detected, and state keeps
  the configured value.
- `laravel_cloud_database_cluster.version` is not returned by the API, so an
  imported cluster leaves it null.
