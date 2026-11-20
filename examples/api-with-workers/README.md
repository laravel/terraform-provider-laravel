# API with queue workers, a scheduler and websockets

A queue-heavy JSON API ("Acme Ledger"). The web tier is small because it only
validates a request and dispatches a job. The work happens on a separate queue
instance, and clients get results over Reverb instead of polling.

## The shape

```
                    ┌─ web        flex.c-1vcpu-512mb, Octane, autoscaled
environment ────────┼─ queue      managed_queue, scales to zero when drained
  production        │               ├─ worker process  ×4  (redis, 3 tries)
                    │               └─ custom process  ×1  (bank feed daemon)
                    └─ scheduler  1 replica, uses_scheduler

cache   Valkey 500mb, eviction_policy = noeviction  (it holds jobs)
reverb  websocket server + application, credentials wired into env vars
```

Three details are easy to get wrong:

- **The queue instance is `type = "managed_queue"`**, so it scales to zero when
  idle. `min_replicas` does not apply to it, and the API rejects
  `min_replicas`/`max_replicas` for `scaling_type = "auto"` generally.
- **The queue cache uses `eviction_policy = "noeviction"`.** The default
  `allkeys-lru` evicts queued jobs under memory pressure without any error.
- **The scheduler is its own instance.** Enable `uses_scheduler` on the web
  tier and every replica fires the same scheduled command.

Reverb credentials (`app_id`, `key`, `secret`, hostname) are read off the
resources and fed into `laravel_cloud_environment_variables`, so nobody
copies them by hand.

## What to change before running

```sh
terraform apply \
  -var 'repository=your-org/your-repo' \
  -var 'region=us-east-2' \
  -var 'domain=api.yourcompany.com'
```

The domain will not verify until you create the records in the
`api_dns_records` output at your DNS provider.

## Run

```sh
export LARAVEL_CLOUD_API_TOKEN="<your-api-token>"
cd examples/api-with-workers
terraform init
terraform plan
terraform apply   # creates real, billable infrastructure
```

To run against a local build of the provider instead of the registry, see the
`dev_overrides` instructions in [`../basic-app/README.md`](../basic-app/README.md).
