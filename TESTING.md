# Testing the Laravel Cloud Terraform provider

## Unit tests

```sh
make test
```

## Acceptance tests

Acceptance tests create real resources in Laravel Cloud. They need a valid API token and incur usage.

### Prerequisites

Set the following environment variables:

```sh
export LARAVEL_CLOUD_API_TOKEN="your-api-token"
```

Optionally override the API base URL (defaults to `https://cloud.laravel.com/api`):

```sh
export LARAVEL_CLOUD_BASE_URL="https://your-api-host/api"
```

### Running all acceptance tests

```sh
make testacc
```

Acceptance tests build from a real source repository, which defaults to
`laravel/laravel` on branch `13.x`. Override either if that repository is not
connected to the account under test:

```sh
export LARAVEL_CLOUD_TEST_REPOSITORY="your-org/your-repo"
export LARAVEL_CLOUD_TEST_BRANCH="main"
```

A few tests need a fixture that not every organization has. Each one is skipped
unless its variable is set:

| Variable | Fixture |
|---|---|
| `LARAVEL_CLOUD_TEST_MONOREPO_REPOSITORY` | A repository with a `backend/` directory |
| `LARAVEL_CLOUD_TEST_MAIN_BRANCH_REPOSITORY` | A repository whose default branch is `main` |
| `LARAVEL_CLOUD_TEST_UNUSED_REPOSITORY` | A repository with default branch `main` that no other application in the organization uses |
| `LARAVEL_CLOUD_TEST_DOMAIN_ZONE` | A real domain zone, such as `example.dev`; the test adds an unverified domain under it |

The database cluster acceptance test is commented out. Provisioning a
cluster routinely takes twenty minutes or more, and neither the databases
inside it nor the cluster itself can be deleted until it finishes, so a
create-then-destroy cycle either blocks the run for a long time or leaves a
billable database behind. The plan tests below cover its behaviour. They
run the same code paths against the in-memory fake in under a
second. Uncomment it in `internal/provider/resource_database_cluster_test.go`
to run it against a real API. `TestAccDatabaseCluster_storageAutoscale` does
run a Laravel MySQL cluster live, with `create_default_database = false` so it
destroys cleanly; it takes under a minute in the test organization.

### Running a specific test

```sh
TF_ACC=1 go test ./internal/provider/ -v -run TestAccApplicationResource_basic -timeout 60m
```

### Debugging

Enable Terraform debug logging:

```sh
TF_LOG=DEBUG make testacc
```

### Cleaning up (sweepers)

Every acceptance test names its resources with the `tfacc-` prefix (use
`testAccName()`). The sweepers delete only resources with that prefix, so they
are safe to run against an organization that also holds real resources:

```sh
go test ./internal/provider -v -sweep=all
```

They delete applications (and with them their environments), WebSocket
servers, caches, buckets (keys first), database users (in any cluster, by
username prefix `tfacc_`), database clusters (databases first) and
secrets.

## Plan-check tests (fake API)

Plan-check tests run a real `terraform plan`/`apply` cycle against an
in-memory fake of the Laravel Cloud API (`internal/provider/mock_cloud_test.go`).
They assert that the plan and applied state match what the configuration asks
for. For example, they check that the `examples/basic-app` config plans cleanly,
that `php_version = "8.4:1"` reads back as `php_major_version = "8.4"`, and
that re-applying the same config changes nothing (no perpetual diff).

Unlike acceptance tests, they create no real resources and need no API token or
network. They need only `TF_ACC=1`, which the testing framework requires before
it runs any plan or apply, and a `terraform` binary on `PATH`.

```sh
make testplan
```

Or directly:

```sh
TF_ACC=1 go test ./internal/provider/ -v -run 'Plan$' -timeout 5m
```

Without `TF_ACC=1` these tests are skipped, so they do not run as part of
`make test`.

### Coverage

Every resource has a plan-check test that creates and applies it against the
fake API, then plans again and expects an empty plan.

> `connection_details` on the cache, database cluster, and database
> restore resources is modeled as a `types.Object`. A Go pointer-struct
> (`*XConnectionModel`) cannot hold an `unknown` value, so the previous modeling
> made `Create`'s `req.Plan.Get` fail with *"Received unknown value, however the
> target type cannot handle unknown values"*, so these resources could be planned
> but not applied. The `types.Object` modeling fixes that; the tests above guard
> against a regression.

## Writing tests

Tests use `terraform-plugin-testing` and follow the usual Terraform provider patterns:

1. Use `testAccPreCheck(t)` in every test's `PreCheck` to check the API token
2. Use `testAccProtoV6ProviderFactories` as the provider factory
3. Name resources with `testAccName()`, which adds the `tfacc-` prefix the sweepers rely on
4. Include both create and import test steps
5. Test updates in a separate test function with multiple steps
