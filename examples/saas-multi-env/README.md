# SaaS with production and staging

A multi-tenant SaaS ("Acme Invoicing") running two environments off one
application.

| Piece | Production | Staging |
|---|---|---|
| Branch | `main` | `develop` |
| Web instance | `flex.c-2vcpu-1gb`, Octane, autoscaled on CPU/memory | `flex-512mb`, fixed at 1 replica, hibernates after 5 minutes |
| Database | `invoicing_production` | `invoicing_staging` |
| Cache | Valkey 250mb | none, uses the database drivers |
| Domain | `invoicing.example.com` + `www` redirect | assigned vanity domain |
| Mail | real | `MAIL_MAILER=log` |

Both databases live in one `laravel_cloud_database_cluster`. That keeps the
bill to a single cluster while still stopping a staging migration from
touching production rows.

## What to change before running

Everything account-specific is in `variables.tf`:

```sh
terraform apply \
  -var 'repository=your-org/your-repo' \
  -var 'region=us-east-2' \
  -var 'domain=app.yourcompany.com'
```

The domain will not verify until you create the records reported in the
`production_dns_records` output at your DNS provider.

## Two things worth knowing

**`laravel_cloud_environment_variables` wins for the keys it lists.** On every
apply it writes those keys, overwriting a value a teammate changed in the
dashboard. Removing a key from the map deletes that variable. Variables added
in the dashboard and never listed here are left alone, and Terraform does not
show them.

**The cluster will not destroy while it holds databases.** `terraform destroy`
fails and names them. Set `force_destroy = true` on the cluster to allow it,
which deletes *every* database in the cluster, including ones Terraform never
created.

## Run

```sh
export LARAVEL_CLOUD_API_TOKEN="<your-api-token>"
cd examples/saas-multi-env
terraform init
terraform plan
terraform apply   # creates real, billable infrastructure
```

To run against a local build of the provider instead of the registry, see the
`dev_overrides` instructions in [`../basic-app/README.md`](../basic-app/README.md).
