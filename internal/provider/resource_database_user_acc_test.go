package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// A database user, created in the same apply as its Laravel MySQL
// cluster (which refuses it while the cluster is still being created), then
// changed in place, imported, and destroyed. A Laravel MySQL cluster with
// create_default_database = false is available in about 30 seconds and
// destroys cleanly; Serverless Postgres does not support database users.
func TestAccDatabaseUser_basic(t *testing.T) {
	name := testAccName()
	username := strings.ReplaceAll(name, "-", "_")
	addr := "laravel_cloud_database_user.test"
	var clusterID, userID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			_, err := testAccClient().GetDatabaseUser(context.Background(), clusterID, userID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("database user %s still exists after destroy (err: %v)", userID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseUserConfig(name, username, "read_only", "[laravel_cloud_database.app.id]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "username", username),
					resource.TestCheckResourceAttr(addr, "type", "read_only"),
					resource.TestCheckResourceAttr(addr, "status", "available"),
					resource.TestCheckResourceAttr(addr, "databases.#", "1"),
					resource.TestCheckResourceAttrPair(addr, "databases.0", "laravel_cloud_database.app", "id"),
					resource.TestCheckResourceAttrSet(addr, "password"),
					func(s *terraform.State) error {
						rs := s.RootModule().Resources[addr].Primary
						clusterID, userID = rs.Attributes["cluster_id"], rs.ID
						return nil
					},
				),
			},
			{
				Config: testAccDatabaseUserConfig(name, username, "read_write", "[laravel_cloud_database.app.id, laravel_cloud_database.reports.id]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "type", "read_write"),
					resource.TestCheckResourceAttr(addr, "databases.#", "2"),
					// Kept from the create response; no read returns it.
					resource.TestCheckResourceAttrSet(addr, "password"),
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					return clusterID + ":" + userID, nil
				},
			},
		},
	})
}

func testAccDatabaseUserConfig(name, username, userType, databases string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_database_cluster" "test" {
  name                    = %[1]q
  type                    = "laravel_mysql"
  version                 = "8.4"
  region                  = "us-east-2"
  create_default_database = false
  force_destroy           = true
  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 0
  })
}

resource "laravel_cloud_database" "app" {
  cluster_id = laravel_cloud_database_cluster.test.id
  name       = "app"
}

resource "laravel_cloud_database" "reports" {
  cluster_id = laravel_cloud_database_cluster.test.id
  name       = "reports"
}

resource "laravel_cloud_database_user" "test" {
  cluster_id = laravel_cloud_database_cluster.test.id
  username   = %[2]q
  type       = %[3]q
  databases  = %[4]s
}
`, name, username, userType, databases)
}
