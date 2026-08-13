# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
