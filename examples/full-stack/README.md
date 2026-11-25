# Full-stack example: every resource

Uses 17 of the 24 resources the provider exposes, with the provider built
locally from this repo. Four of them, the database resources, are commented out.

It is split into tiers so a default run stays cheap and safe:

| Tier | Resources | When |
|------|-----------|------|
| **1: core** (`main.tf`) | application, environment, instance, environment_variables, cache, storage_bucket, storage_bucket_key, websocket_server, websocket_application | created by default |
| **2: side-effecting** (`side-effecting.tf`) | deployment, command | opt-in with `enable_side_effecting=true` |
| **commented out** | database_cluster, database, database_snapshot, database_restore | uncomment in `main.tf` / `side-effecting.tf` to use |
| **conditional** (`main.tf`) | background_process, domain | opt-in with `enable_api_blocked=true` |

Tier 2 is gated because those resources run real operations (a deploy, an
artisan command) or need live state (a deployed env, a fresh PITR window).
See the header of `side-effecting.tf` for the caveats.

## Run

```sh
# Plan the core tier (no resources created, validates auth + every schema)
./run.sh

# Create the core resources, then tear them down
./run.sh apply
./run.sh destroy

# Include the 2 side-effecting resources
SIDE_EFFECTING=1 ./run.sh plan
SIDE_EFFECTING=1 ./run.sh apply
```

`run.sh` builds the provider binary, generates a `dev_overrides` config
pointing at it, loads your token, and runs terraform. Extra arguments pass through,
for example `./run.sh apply -auto-approve`.

The token is not hardcoded. `run.sh` requires `LARAVEL_CLOUD_API_TOKEN`.
Either export it, or put it in an untracked `.env.local` (gitignored) which
`run.sh` sources automatically:

```sh
echo 'export LARAVEL_CLOUD_API_TOKEN="<your-api-token>"' > .env.local
```

> Even with Tier 2 off, `terraform validate` / `plan` still type-checks the
> gated config, so every resource's schema is exercised on every run; only the
> *apply* of Tier 2 is opt-in.

## Notes

- `variables.tf` exposes `prefix` (default `tf-fullstack`), `app_region`,
  `domain_name`, `enable_api_blocked`, and `enable_side_effecting`. Change
  `prefix` if a name collides. Storage bucket names may need to be globally
  unique.
- The database cluster, its database, and the snapshot and restore that depend
  on it are commented out. Provisioning a cluster routinely takes twenty
  minutes or more, and neither the databases inside it nor the cluster itself
  can be deleted until it finishes, so a plain apply/destroy cycle would either
  block for a long time or leave a billable database behind. The configuration
  is kept as a worked reference. Uncomment the block in `main.tf`, the
  `database_cluster_status` output, and the two resources in
  `side-effecting.tf` to exercise them.
- `database_cluster.config` must list every key the API echoes back, with
  exactly the value it stores. `size` comes from the enum in
  `GET /databases/types` (`mysql-flex-512mb`, …), not from the instance sizes.
  A wrong-but-accepted value is normalized server-side and turns into a
  permanent diff.
- Attribute values mirror the plan-check tests
  (`internal/provider/resources_plan_test.go`). If the API rejects a region,
  type, or size, adjust it in `main.tf`.
- Keep your token in `.env.local` (gitignored) only. Never commit it, and
  rotate it when you're done.
