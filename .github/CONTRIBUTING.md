# Contributing

The general Laravel contribution guide can be found in the [Laravel documentation](https://laravel.com/docs/contributions).

The notes below cover what is specific to this Terraform provider.

## Development

Requires Go >= 1.25 and Terraform >= 1.0.

```sh
make build     # compile the provider binary
make install   # install into ~/.terraform.d/plugins for local dev
```

To run a local build against a real configuration, add a `dev_overrides` block
to `~/.terraformrc` pointing at your checkout:

```hcl
provider_installation {
  dev_overrides {
    "laravel/laravel" = "/path/to/your/checkout"
  }
  direct {}
}
```

With an override active, `terraform init` is not needed — Terraform prints a
warning saying so, which is expected.

## Tests

```sh
make test      # unit tests; no network, no credentials
make testplan  # plan/apply against an in-memory fake API; needs a terraform binary
make testacc   # acceptance tests; creates REAL resources and incurs usage
make lint      # golangci-lint
```

`make test` and `make testplan` are what CI gates on for a pull request. Please
make sure both pass before opening one. See [TESTING.md](../TESTING.md) for the
full breakdown, including how the fake API works.

## Documentation

Everything under `docs/` is generated from the provider schema and the files in
`examples/`. Do not edit it by hand — change the schema descriptions or the
example, then regenerate:

```sh
make generate
```

CI fails if the committed docs differ from what the generator produces.

## Pull requests

- Target `main`.
- Include a test that fails without your change.
- Update `CHANGELOG.md` under `## [Unreleased]`.
- Adding or changing a resource attribute means updating its schema
  `Description` — that text is what renders on the Terraform Registry.
