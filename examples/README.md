# Examples

## Scenarios

Complete configurations you can read top to bottom, each modelled on a real
deployment shape.

| Directory | What it shows |
|---|---|
| [`basic-app/`](basic-app) | The smallest useful config: one application, one production environment. |
| [`saas-multi-env/`](saas-multi-env) | A SaaS with production and staging off one app — shared database cluster, separate databases, cache and custom domain on production only. |
| [`api-with-workers/`](api-with-workers) | A queue-heavy API — managed queue instance, worker and custom background processes, a dedicated scheduler, and a Reverb websocket tier. |
| [`full-stack/`](full-stack) | Every resource the provider exposes, tiered so a default run stays cheap. Built for exercising the provider, not for copying. |

All of them create real, billable infrastructure on apply. `terraform plan` is
free.

## Registry snippets

`resources/` and `data-sources/` hold the one-resource snippets embedded into
the [registry documentation](https://registry.terraform.io/providers/laravel/laravel/latest/docs)
by `tfplugindocs`. They are intentionally minimal. Edit them and run
`make generate` to refresh `docs/`.
