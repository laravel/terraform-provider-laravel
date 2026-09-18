# Terraform Provider for Laravel Cloud

Manage [Laravel Cloud](https://cloud.laravel.com) infrastructure as code.

> **BETA RELEASE**: The Terraform Provider for the Laravel Cloud API is in Early Access and
> subject to change, so provider resources and attributes may change between
> releases.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25 (to build the provider)

## Installation

Add the provider to your Terraform configuration and run `terraform init`:

```hcl
terraform {
  required_providers {
    laravel = {
      source  = "laravel/laravel"
      version = "~> 1.0"
    }
  }
}

provider "laravel" {
  # Set token here or via the LARAVEL_CLOUD_API_TOKEN env var
}
```

Then run:

```sh
export LARAVEL_CLOUD_API_TOKEN="your-laravel-cloud-api-token"
terraform init
terraform plan
```

## Authentication

The provider requires a Laravel Cloud API token. Set it via environment variable:

```sh
export LARAVEL_CLOUD_API_TOKEN="your-token-here"
```

Or configure it directly in the provider block:

```hcl
provider "laravel" {
  token = "your-token-here"
}
```

## Resources

| Resource | Description |
|---|---|
| `laravel_cloud_application` | Application |
| `laravel_cloud_environment` | Environment within an application |
| `laravel_cloud_instance` | Compute instance within an environment |
| `laravel_cloud_domain` | Custom domain for an environment |
| `laravel_cloud_database_cluster` | Database cluster (MySQL, Postgres, Neon) |
| `laravel_cloud_database` | Database schema within a cluster |
| `laravel_cloud_database_snapshot` | Point-in-time database snapshot |
| `laravel_cloud_database_restore` | Restore a database from a snapshot |
| `laravel_cloud_cache` | Cache store (Valkey / Redis) |
| `laravel_cloud_storage_bucket` | Object storage bucket |
| `laravel_cloud_storage_bucket_key` | Access key for a storage bucket |
| `laravel_cloud_background_process` | Background worker process |
| `laravel_cloud_environment_variables` | Environment variables for an environment |
| `laravel_cloud_deployment` | Deployment trigger |
| `laravel_cloud_command` | Run a command in an environment |
| `laravel_cloud_websocket_server` | WebSocket server |
| `laravel_cloud_websocket_application` | WebSocket application |

## Data Sources

| Data Source | Description |
|---|---|
| `laravel_cloud_organization` | Current organization details |
| `laravel_cloud_dedicated_clusters` | Available dedicated clusters |
| `laravel_cloud_instance_sizes` | Available instance sizes |
| `laravel_cloud_database_types` | Available database types |
| `laravel_cloud_cache_types` | Available cache types |
| `laravel_cloud_regions` | Available regions |
| `laravel_cloud_ip_addresses` | IP addresses for allowlisting |

## Development

### Building

```sh
make build     # compile the provider binary
make install   # install into ~/.terraform.d/plugins for local dev
```

### Testing

```sh
make test      # unit tests
make testacc   # acceptance tests (requires LARAVEL_CLOUD_API_TOKEN)
make lint      # golangci-lint
```

See [`examples/`](examples/) for sample Terraform configurations.
