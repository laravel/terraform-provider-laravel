#!/usr/bin/env bash
# Build the provider and run a terraform command against the full-stack example
# using the locally-built binary.
#
# Usage:
#   ./run.sh                 # terraform plan (core tier only)
#   ./run.sh apply           # create the core resources
#   ./run.sh destroy         # tear them down
#   SIDE_EFFECTING=1 ./run.sh plan     # also include the 4 tier-2 resources
#   SIDE_EFFECTING=1 ./run.sh apply    # ...and actually run them
#
# Any extra args are passed straight through to terraform, e.g.
#   ./run.sh apply -auto-approve
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
HERE="$ROOT/examples/full-stack"

# API token. Sourced from an untracked .env.local (gitignored) if present,
# otherwise must be exported yourself. Never hardcode it in this tracked file.
#   echo 'export LARAVEL_CLOUD_API_TOKEN="<token>"' > examples/full-stack/.env.local
[ -f "$HERE/.env.local" ] && . "$HERE/.env.local"
: "${LARAVEL_CLOUD_API_TOKEN:?Set LARAVEL_CLOUD_API_TOKEN (export it or put it in examples/full-stack/.env.local)}"

# 1. Build the provider binary the dev_override points at.
echo "==> Building provider binary..."
(cd "$ROOT" && go build -o terraform-provider-laravel .)

# 2. Resolve the provider from the local build (no `terraform init` needed).
#    Generated here rather than committed so it works in any checkout.
TFRC="$(mktemp "${TMPDIR:-/tmp}/laravel-dev-overrides.XXXXXX")"
trap 'rm -f "$TFRC"' EXIT
cat >"$TFRC" <<EOF
provider_installation {
  dev_overrides {
    "laravel/laravel" = "$ROOT"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$TFRC"

# 3. Run terraform.
ACTION="${1:-plan}"
[ $# -gt 0 ] && shift

VAR_ARGS=()
if [ "${SIDE_EFFECTING:-0}" = "1" ]; then
  VAR_ARGS+=(-var "enable_side_effecting=true")
  echo "==> Tier 2 (side-effecting) resources ENABLED"
fi

cd "$HERE"
echo "==> terraform $ACTION"
# Deliberately not `exec`: that would replace this shell and skip the EXIT
# trap above, leaking the generated tfrc into $TMPDIR on every run. `set -e`
# propagates terraform's exit status.
# ${arr[@]+...} guards empty-array expansion under `set -u` on bash 3.2 (macOS).
terraform "$ACTION" ${VAR_ARGS[@]+"${VAR_ARGS[@]}"} "$@"
