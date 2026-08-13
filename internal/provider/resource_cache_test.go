package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCacheResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccCacheConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "name", rName),
					resource.TestCheckResourceAttrSet("laravel_cloud_cache.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "type", "laravel_valkey"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "region", "us-east-2"),
					resource.TestCheckResourceAttr("laravel_cloud_cache.test", "size", "valkey-pro.250mb"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_cache.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCacheConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_cache" "test" {
  name                 = %[1]q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-pro.250mb"
  auto_upgrade_enabled = true
  is_public            = false
}
`, name)
}
