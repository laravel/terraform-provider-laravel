# A prefix applied to every resource name so a full run is easy to spot and
# clean up in the dashboard, and so repeat runs from different machines
# don't collide. Bucket names in particular may need to be globally unique --
# bump this if `apply` reports a name conflict.
variable "prefix" {
  type    = string
  default = "tf-fullstack"
}

# Region for the application/environment. Database, cache and websocket
# resources pin their own regions below (us-east-1) to match what the API
# currently offers; adjust if the API rejects them.
variable "app_region" {
  type    = string
  default = "us-east-2"
}

# A hostname for the laravel_cloud_domain resource. It will never verify
# against a domain you don't control, but the create call still exercises the
# resource. Override with a domain you own to test verification end to end.
variable "domain_name" {
  type    = string
  default = "tf-fullstack.example.com"
}

# Toggle for background_process and domain. These depend on API behaviour that
# is not available on every deployment, so they are off by default and a plain
# `apply` reflects what the API can reliably do. Enable them with:
#   terraform apply -var enable_api_blocked=true
variable "enable_api_blocked" {
  type    = bool
  default = false
}

# Tier 2 toggle. Off by default: a plain `apply` creates only the 13 core
# resources, all of which are ordinary CRUD. Set true to also exercise the
# four side-effecting resources (deployment, command, snapshot, restore),
# which run real operations and need a deployed environment / a live PITR
# window -- see side-effecting.tf.
variable "enable_side_effecting" {
  type    = bool
  default = false
}

# Toggle for the database cluster and the database inside it. Off by default:
# provisioning a cluster routinely takes twenty minutes or more, and nothing in
# it can be deleted until it finishes, so a plain apply/destroy cycle would
# either block for a long time or leave a billable database behind. Enable with:
#   terraform apply -var enable_database=true
variable "enable_database" {
  type    = bool
  default = false
}
