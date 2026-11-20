package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationResource_basic(t *testing.T) {
	rName := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccApplicationConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "name", rName),
					resource.TestCheckResourceAttrSet("laravel_cloud_application.test", "id"),
					resource.TestCheckResourceAttr("laravel_cloud_application.test", "region", "us-east-2"),
				),
			},
			// Import
			{
				ResourceName:      "laravel_cloud_application.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccApplicationConfig(name string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}
`, name, testAccRepository())
}
