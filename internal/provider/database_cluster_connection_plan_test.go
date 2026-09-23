package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The tests below guard a defect reported against an imported database
// cluster: the first plan, with nothing changed, printed "this attribute value
// will no longer be marked as sensitive after applying this change" above the
// connection password.
//
// connection_details is Computed with no plan modifier, so any in-place update
// -- and an import always plans one, for version and force_destroy, which the
// API never reports -- planned the whole object as unknown. An unknown object
// cannot carry the sensitive mark on its nested password, which is what
// Terraform warns about.

const clusterAddr = "laravel_cloud_database_cluster.main"

var clusterPasswordPath = tfjsonpath.New("connection_details").AtMapKey("password")

func connectionCluster(baseURL, storage, extra string) string {
	return providerCfg(baseURL) + `
resource "laravel_cloud_database_cluster" "main" {
  name    = "conn-db"
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"
  ` + extra + `

  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = ` + storage + `
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 7
    suspend_seconds          = 0
  })
}
`
}

func TestDatabaseClusterImportKeepsPasswordSensitivePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	config := connectionCluster(baseURL, "10", "force_destroy = true")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:          config,
				ResourceName:    clusterAddr,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				// version and force_destroy are never reported, so the import
				// still plans recording them.
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}

// An update that leaves config alone cannot change the connection, so the
// recorded details are kept rather than planned as unknown.
func TestDatabaseClusterConnectionKeptWithoutConfigChangePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: connectionCluster(baseURL, "10", "")},
			{
				Config: connectionCluster(baseURL, "10", "force_destroy = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(clusterAddr,
							tfjsonpath.New("connection_details").AtMapKey("hostname"),
							knownvalue.StringExact("db.test.local")),
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}

// A config change may move the connection (is_public changes the hostname), so
// the details are unknown -- but the password must stay marked sensitive, and
// apply must accept whatever the API reports.
func TestDatabaseClusterConfigChangeKeepsPasswordSensitivePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: connectionCluster(baseURL, "10", "force_destroy = true")},
			{
				Config: connectionCluster(baseURL, "20", "force_destroy = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(clusterAddr, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(clusterAddr,
							tfjsonpath.New("connection_details").AtMapKey("hostname")),
						plancheck.ExpectSensitiveValue(clusterAddr, clusterPasswordPath),
					},
				},
			},
		},
	})
}
