# API with queue workers, a scheduler and websockets

A queue-heavy JSON API ("Acme Ledger"). The web tier is small because it only
validates a request and dispatches a job. The work happens on a separate queue
instance, and clients get results over Reverb instead of polling.

## The shape

```
                    ┌─ web             app instance, flex-1gb, Octane, autoscaled
environment ────────┼─ ledger-default  managed queue (default), mq.flex.512mb, 0-10 workers
  production        ├─ ledger-high     managed queue, mq.flex.256mb, 0-5 workers
                    └─ scheduler       1 replica, uses_scheduler
                                         └─ custom process ×1 (bank feed daemon)

cache   Valkey 1gb, cache store and sessions
reverb  websocket server + application, credentials wired into env vars
```

Three details are easy to get wrong:

- **Queues are managed queues on current-generation `mq.*` sizes.** Each
  queue runs its own workers, which scale on queue depth between
  `min_replicas` and `max_replicas` and to zero when it drains. Laravel Cloud
  sets `QUEUE_CONNECTION=cloud`, so the example does not set it. One managed
  queue handles one queue name: jobs dispatched without a name go to the
  queue with `is_default = true`, and payment jobs use
  `->onQueue('ledger-high')`.
- **Long-running daemons do not belong on a managed queue.** The bank feed
  stream is a custom background process on the single-replica scheduler
  instance, so exactly one copy runs.
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
