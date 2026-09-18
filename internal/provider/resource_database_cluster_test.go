package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatabaseClusterResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccDatabaseClusterConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "name", rName),
					resource.TestCheckResourceAttrSet("laravel_cloud_database_cluster.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "type", "laravel_mysql"),
					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "region", "us-east-2"),
					resource.TestCheckResourceAttrSet("laravel_cloud_database_cluster.test", "status"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_database_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
				// version is a create-only request field: the API accepts it and
				// never reports it back, so an imported cluster cannot recover
				// it. config is ignored because an imported cluster has no
				// configured JSON to preserve, so it takes the API's full
				// effective configuration including defaults the original
				// config never set.
				ImportStateVerifyIgnore: []string{"version", "config"},
			},
		},
	})
}

func testAccDatabaseClusterConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_database_cluster" "test" {
  name   = %[1]q
  type    = "laravel_mysql"
  version = "8.4"
  region  = "us-east-2"
  config = jsonencode({
    size                     = "mysql-flex-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 0
  })
}
`, name)
}
