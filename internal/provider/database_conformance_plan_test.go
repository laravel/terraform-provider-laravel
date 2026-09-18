package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestDatabaseClusterRetiredTypePlan guards the database type round-trip.
//
// The retired type identifiers (laravel_mysql_84 and friends) bake the engine
// version into the type and are still accepted on create, but the API always
// reports the *base* type back -- DatabaseType is enum ["laravel_mysql",
// "aws_rds_mysql", "aws_rds_postgres", "neon_serverless_postgres"].
//
// Writing that base value straight into state made it differ from the config on
// every read, and because `type` carries RequiresReplace, the very next plan
// proposed destroying and recreating a live database cluster. The configured
// spelling must survive the round-trip.
func TestDatabaseClusterRetiredTypePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := databaseClusterConfig(baseURL, `type = "laravel_mysql_84"`)

	const addr = "laravel_cloud_database_cluster.db"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"),
						knownvalue.StringExact("laravel_mysql_84")),
				},
			},
			// The load-bearing assertion: no proposed replacement of a live
			// database cluster on an unchanged config.
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestDatabaseClusterVersionPlan covers the current type identifiers, which
// require a separate version argument. Before this existed, the provider could
// only ever create clusters via the retired identifiers.
func TestDatabaseClusterVersionPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := databaseClusterConfig(baseURL, "type = \"laravel_mysql\"\n  version = \"8.4\"")

	const addr = "laravel_cloud_database_cluster.db"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("type"),
						knownvalue.StringExact("laravel_mysql")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("version"),
						knownvalue.StringExact("8.4")),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestDatabaseSnapshotPendingStoragePlan covers a snapshot that is still
// pending when create returns, which is the normal case.
//
// storage_bytes is Computed and nullable, and it was assigned only when the API
// returned a non-nil value. A pending snapshot has no size yet, so the
// attribute stayed unknown after apply and Terraform aborted the whole apply
// with "provider produced inconsistent result after apply".
func TestDatabaseSnapshotPendingStoragePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := databaseSnapshotPendingConfig(baseURL)

	const addr = "laravel_cloud_database_snapshot.snap"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("storage_bytes"), knownvalue.Null()),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func databaseClusterConfig(baseURL, typeAndVersion string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_database_cluster" "db" {
  name   = "primary"
  region = "us-east-2"
  # A test fixture opts in so the auto-created database does not block cleanup.
  force_destroy = true
  %[2]s
  config = jsonencode({ size = "mysql-flex-1gb" })
}
`, baseURL, typeAndVersion)
}

func databaseSnapshotPendingConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_database_cluster" "db" {
  name          = "primary"
  region        = "us-east-2"
  type          = "laravel_mysql_84"
  force_destroy = true
  config        = jsonencode({ size = "mysql-flex-1gb" })
}

resource "laravel_cloud_database_snapshot" "snap" {
  cluster_id = laravel_cloud_database_cluster.db.id
  name                = "nightly"
}
`, baseURL)
}

// TestDatabaseClusterConfigDefaultsPlan guards the config round-trip.
//
// config is a Required, user-authored JSON string, but the API echoes back the
// full effective configuration including keys the caller never set --
// suspend_seconds and storage_autoscale_max_gb among them. Writing that whole
// object into state made every plan propose deleting those keys, and the API
// always added them back, so the diff could never converge. Observed against a
// live API, where the refresh plan after a clean apply was not empty.
func TestDatabaseClusterConfigDefaultsPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := databaseClusterConfig(baseURL, `type = "laravel_mysql_84"`)

	const addr = "laravel_cloud_database_cluster.db"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					// The configured JSON survives verbatim; the API's extra
					// defaults are not merged into the user's attribute.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("config"),
						knownvalue.StringExact(`{"size":"mysql-flex-1gb"}`)),
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestDatabaseClusterForceDestroyPlan pins the guard on the most destructive
// path in the provider.
//
// The API refuses to delete a cluster while any database is attached, and a
// cluster always carries at least the one created with it, so a destroy cannot
// succeed without removing them. Removing them implicitly would be worse than
// the failure it replaces: the cluster may hold databases created outside
// Terraform, this resource's state does not track them, and
// lifecycle.prevent_destroy could not protect them because they are not
// resources in state. So the sweep is opt-in.
//
// Without force_destroy, destroy must fail and name what is in the way.
// TestDatabaseClusterForceDestroyPlan pins the guard on the most destructive
// path in the provider.
//
// The API refuses to delete a cluster while any database is attached, and the
// platform creates one inside every new cluster, so a destroy cannot succeed
// without removing them. Removing them implicitly would be worse than the
// failure it replaces: a cluster may hold databases created outside Terraform,
// this resource's state does not track them, and lifecycle.prevent_destroy
// could not protect them because they are not resources in state. So the sweep
// is opt-in, and without it the destroy must fail and name what is in the way.
func TestDatabaseClusterForceDestroyPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: databaseClusterNoForceDestroyConfig(baseURL),
			},
			{
				Config:      databaseClusterNoForceDestroyConfig(baseURL),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`still contains databases`),
			},
			// Opting in lets the destroy proceed, and leaves the harness able to
			// clean up after itself.
			{
				Config: databaseClusterForceDestroyConfig(baseURL),
			},
		},
	})
}

func databaseClusterForceDestroyConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_database_cluster" "db" {
  name          = "primary"
  region        = "us-east-2"
  type          = "laravel_mysql_84"
  force_destroy = true
  config        = jsonencode({ size = "mysql-flex-1gb" })
}
`, baseURL)
}

// databaseClusterNoForceDestroyConfig deliberately omits force_destroy, which
// is the default and the case the guard protects.
func databaseClusterNoForceDestroyConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_database_cluster" "db" {
  name   = "primary"
  region = "us-east-2"
  type   = "laravel_mysql_84"
  config = jsonencode({ size = "mysql-flex-1gb" })
}
`, baseURL)
}
