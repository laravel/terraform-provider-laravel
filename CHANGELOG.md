# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

This release corrects the provider's mapping onto the Laravel Cloud API. Every
change below was verified against the published OpenAPI spec, and most were
reproduced against a live API.

Several corrections are breaking, because the behaviour they replace was wrong
in a way configurations had been written around. **Read "Breaking changes"
before upgrading.**

### Breaking changes

- `laravel_cloud_instance.polling_interval` is now read-only. The API accepts it
  in neither the create nor the update request, so setting it never had any
  effect. A configuration that sets it now fails at plan time with "Cannot set
  value for this attribute as the provider has marked it as read-only" — remove
  the line.
- `laravel_cloud_instance.min_replicas` and `max_replicas` are no longer
  required, and setting either alongside `scaling_type = "auto"` is now rejected
  during validation. The API documents them as "rejected when used with auto"
  and 422s such a request, so these configurations could never apply; the error
  has simply moved from apply time to plan time. Because both attributes used to
  be required, every existing `auto` configuration sets them and must drop them.
- `laravel_cloud_domain`: `www_redirect`, `wildcard_enabled`, `allow_downtime`
  and `cloudflare_strategy` now force replacement. The update endpoint accepts
  `verification_method` and nothing else, so changing any of them used to report
  success, write the new value to state, and leave the live domain untouched.
  They now do what the plan says — which means **changing one destroys and
  recreates a live domain**, and the replacement is destroy-before-create, so the
  domain stops serving until it re-verifies. Review plans involving these
  attributes carefully, and consider `lifecycle { prevent_destroy = true }` on
  production domains.
- `laravel_cloud_database_cluster` cannot be destroyed while it contains
  databases unless the new `force_destroy` is set. The API refuses to delete a
  cluster with databases attached and a cluster always carries at least the one
  created with it, so destroys previously failed with a confusing error after
  five minutes of retries. Destroy now fails immediately and names the databases
  in the way. Setting `force_destroy = true` deletes every database in the
  cluster, including any this configuration does not manage.
- `laravel_cloud_websocket_application.allowed_origins` is now also computed,
  because the API always returns a list there and a configuration that omitted
  the attribute failed to apply. As a consequence, removing the attribute from a
  configuration no longer clears the origins; assign `allowed_origins = []`
  instead.

### Fixed

- Environment create no longer wipes `build_command` and `deploy_command`. They
  are optional and computed, so a value the user did not set arrives as unknown
  rather than null; the guard tested only for null, and the empty string it then
  sent made the API clear both.
- Six environment attributes -- `color`, `timeout`, `sleep_timeout`,
  `shutdown_timeout`, `uses_purge_edge_cache_on_deploy` and `cache_strategy` --
  are accepted by the update endpoint but never returned. They were decoded as
  response fields, so each refresh overwrote the configured value with a zero
  value and every plan proposed writing it again, forever.
- `cache_strategy` is now read from `network_settings.cache.strategy`, the only
  place the API reports it.
- `uses_deploy_hook`, `uses_vanity_domain` and `uses_purge_edge_cache_on_deploy`
  were never sent, so configuring them reported success while the API was never
  told.
- `php_major_version` is no longer stale after create: it is taken from the
  response to the settings update rather than the create response, which still
  reports the platform default.
- Detaching a database or cache on environment delete sends null rather than an
  empty string, and a failure is reported instead of swallowed.
- `laravel_cloud_database_cluster.config` no longer produces a diff that cannot
  converge. The API echoes back the full effective configuration, including keys
  the caller never set; only the keys the configuration specifies are compared.
- A cluster created with a retired type identifier such as `laravel_mysql_84` no
  longer proposes replacing itself on the next plan. The API reports the base
  type, which differed from the configured value on every read.
- `laravel_cloud_database_cluster.version` is now supported. It is required by
  the API for the current type identifiers, so only the retired ones worked.
- `laravel_cloud_database_snapshot.storage_bytes` no longer breaks every create.
  A pending snapshot reports no size, and the attribute was left unknown, which
  Terraform rejects after apply.
- `laravel_cloud_websocket_application` no longer discards `key` and `secret`.
  They are returned only when the application is created, and a later read
  replaced them with empty strings.
- `laravel_cloud_cache` connection details are no longer empty after create. The
  create response describes a cache that is still provisioning, so create now
  waits for the connection to be reported, and its fields distinguish "not
  reported yet" from an empty value.
- `laravel_cloud_cache` can be destroyed immediately after being created; the
  delete now retries while the cache is still provisioning instead of failing
  and leaving it behind.
- Importing an instance, domain or background process recovers its parent id.
  The relationships block is only returned when explicitly requested, so the
  parent was empty and, because it forces replacement, the next plan proposed
  destroying the resource that had just been imported.
- `laravel_cloud_instance.queue_status` is no longer stored with quote
  characters around it.
- `source_control_provider_type` accepts `gitlab_self_hosted`.
- `LARAVEL_CLOUD_BASE_URL` is now honoured. It has always been documented, but
  nothing read it, so the provider could not be pointed at another host.

### Added

- `laravel_cloud_edge_networks` data source.
- `laravel_cloud_environment.vanity_domain` is now writable, through the API's
  dedicated endpoint.
- `laravel_cloud_database_cluster.version` and `force_destroy`.
- `laravel_cloud_instance_sizes` reports managed-queue sizes, which were
  dropped entirely, and `cpu_count` is a number because managed-queue sizes
  express fractional vCPUs.
- `laravel_cloud_database_types` reports `versions`, needed to supply the
  cluster `version` argument.
- `laravel_cloud_domain` reports `dns_records`, `stage`, `action_required` and
  `last_verified_at`. Without the records there was no way to complete a domain
  setup from Terraform.

### Known limitations

- The write-only environment attributes listed above cannot be refreshed,
  because the API does not report them. Changing one outside Terraform is not
  detected, and state keeps the configured value.
- `laravel_cloud_database_cluster.version` is not returned by the API, so an
  imported cluster leaves it null.


## [1.0.0]

### Added

- Initial release of the Terraform provider for Laravel Cloud.
- Resources: `application`, `environment`, `instance`, `domain`,
  `database_cluster`, `database`, `database_snapshot`, `database_restore`,
  `cache`, `storage_bucket`, `storage_bucket_key`, `background_process`,
  `environment_variables`, `deployment`, `command`, `websocket_server`,
  `websocket_application`.
- Data sources: `organization`, `dedicated_clusters`, `instance_sizes`,
  `database_types`, `cache_types`, `regions`, `ip_addresses`.
