# Testing the Laravel Cloud Terraform Provider

## Unit Tests

```sh
make test
```

## Acceptance Tests

Acceptance tests create **real resources** in Laravel Cloud. They require a valid API token and will incur usage.

### Prerequisites

Set the following environment variables:

```sh
export LARAVEL_CLOUD_API_TOKEN="your-api-token"
```

Optionally override the API base URL (defaults to `https://cloud.laravel.com/api`):

```sh
export LARAVEL_CLOUD_BASE_URL="https://your-api-host/api"
```

### Running All Acceptance Tests

```sh
make testacc
```

### Running a Specific Test

```sh
TF_ACC=1 go test ./internal/provider/ -v -run TestAccApplicationResource_basic -timeout 60m
```

### Debugging

Enable Terraform debug logging:

```sh
TF_LOG=DEBUG make testacc
```

## Plan-Check Tests (fake API)

Plan-check tests run a **real** `terraform plan`/`apply` cycle against an
in-memory fake of the Laravel Cloud API (`internal/provider/mock_cloud_test.go`).
They assert that the plan and applied state match what the configuration asks
for — for example, that the `examples/basic-app` config plans cleanly, that
`php_version = "8.4:1"` is read back as `php_major_version = "8.4"`, and that
re-applying the same config is a no-op (no perpetual diff).

Unlike acceptance tests, they **create no real resources** and need **no API
token or network** — only `TF_ACC=1` (required by the testing framework to run
any plan/apply) and a `terraform` binary on `PATH`.

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

Every resource has a plan-check test that runs a full create → apply →
re-plan (empty-plan) cycle against the fake API.

> Note: `connection_details` on the cache, database cluster, and database
> restore resources is modeled as a `types.Object`. A Go pointer-struct
> (`*XConnectionModel`) cannot hold an `unknown` value, so the previous modeling
> made `Create`'s `req.Plan.Get` fail with *"Received unknown value, however the
> target type cannot handle unknown values"* — these resources could be planned
> but not applied. The `types.Object` modeling fixes that; the tests above guard
> against a regression.

## Writing Tests

Tests follow the standard Terraform provider testing patterns using `terraform-plugin-testing`:

1. Use `testAccPreCheck(t)` in every test's `PreCheck` to validate the API token
2. Use `testAccProtoV6ProviderFactories` as the provider factory
3. Generate unique resource names with `acctest.RandString()`
4. Include both create and import test steps
5. Test updates in a separate test function with multiple steps
