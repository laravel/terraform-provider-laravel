# Basic app example

Creates a single Laravel Cloud application and its production environment.

## Run against a released provider

```sh
export LARAVEL_CLOUD_API_TOKEN="<your-api-token>"
cd examples/basic-app
terraform init
terraform plan
terraform apply
```

## Run against a local build

When developing the provider, use a `dev_overrides` block so Terraform resolves
`laravel/laravel` to your local binary instead of the registry. Add this to
`~/.terraformrc` (or `%APPDATA%\terraform.rc` on Windows), pointing at your
checkout:

```hcl
provider_installation {
  dev_overrides {
    "laravel/laravel" = "/path/to/your/checkout"
  }
  direct {}
}
```

Then:

```sh
go build -o terraform-provider-laravel .   # from the repo root
export LARAVEL_CLOUD_API_TOKEN="<your-api-token>"
cd examples/basic-app
terraform plan
```

No `terraform init` is needed while a `dev_overrides` block is active —
Terraform prints a warning saying so, which is expected.
