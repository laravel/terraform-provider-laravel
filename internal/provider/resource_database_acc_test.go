package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// A database created in the same apply as its cluster, and a cluster
// destroyed right after its databases.
//
// The API refuses both while the cluster is busy -- still being created, or
// updating after a database was deleted -- with a 422 the provider has to
// recognise and retry. Serverless Postgres is used because it provisions in
// seconds; a MySQL cluster takes twenty minutes or more.
func TestAccDatabase_createWithCluster(t *testing.T) {
	name := testAccName()
	var clusterID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			_, err := testAccClient().GetDatabaseCluster(context.Background(), clusterID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("database cluster %s still exists after destroy (err: %v)", clusterID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseWithClusterConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_database.app", "name", "app_db"),
					resource.TestCheckResourceAttrSet("laravel_cloud_database.app", "id"),
					func(s *terraform.State) error {
						clusterID = s.RootModule().Resources["laravel_cloud_database_cluster.test"].Primary.ID
						return nil
					},
				),
			},
		},
	})
}

func testAccDatabaseWithClusterConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_database_cluster" "test" {
  name          = %[1]q
  type          = "neon_serverless_postgres"
  version       = "17"
  region        = "us-east-2"
  force_destroy = true
  config = jsonencode({
    cu_min          = 0.25
    cu_max          = 0.25
    suspend_seconds = 300
    retention_days  = 1
  })
}

resource "laravel_cloud_database" "app" {
  cluster_id = laravel_cloud_database_cluster.test.id
  name       = "app_db"
}
`, name)
}
