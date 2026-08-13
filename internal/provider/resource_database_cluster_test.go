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
					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "type", "laravel_mysql_8"),
					resource.TestCheckResourceAttr("laravel_cloud_database_cluster.test", "region", "us-east-2"),
					resource.TestCheckResourceAttrSet("laravel_cloud_database_cluster.test", "status"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_database_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
				// config JSON ordering may differ
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

func testAccDatabaseClusterConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_database_cluster" "test" {
  name   = %[1]q
  type   = "laravel_mysql_8"
  region = "us-east-2"
  config = jsonencode({
    size                     = "db-flex.m-1vcpu-512mb"
    storage                  = 10
    is_public                = false
    uses_scheduled_snapshots = false
    retention_days           = 0
  })
}
`, name)
}
