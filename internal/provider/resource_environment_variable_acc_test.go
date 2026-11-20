package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// TestAccEnvironmentVariables_emptyMap checks that variables = {} applies.
// The API refuses to store an empty list (it takes 1 to 210 variables), so
// the provider must not send one.
func TestAccEnvironmentVariables_emptyMap(t *testing.T) {
	name := testAccName()
	addr := "laravel_cloud_environment_variables.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentVariablesConfig(name, ""),
				Check:  resource.TestCheckResourceAttr(addr, "variables.%", "0"),
			},
			{
				Config: testAccEnvironmentVariablesConfig(name, `TFACC_ONE = "1"`),
				Check:  resource.TestCheckResourceAttr(addr, "variables.%", "1"),
			},
			{
				// Removing the last key deletes it and stores nothing.
				Config: testAccEnvironmentVariablesConfig(name, ""),
				Check:  resource.TestCheckResourceAttr(addr, "variables.%", "0"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccEnvironmentVariables_destroyAfterExternalRemoval checks that destroy
// still works when a managed variable was deleted outside Terraform and the
// destroy does not refresh first. The API's delete is atomic and refuses the
// whole request with a 422 when any key is missing. The test
// framework destroys with -refresh=false, so the final destroy is the check.
func TestAccEnvironmentVariables_destroyAfterExternalRemoval(t *testing.T) {
	name := testAccName()
	addr := "laravel_cloud_environment_variables.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentVariablesConfig(name, `
    TFACC_ONE = "1"
    TFACC_TWO = "2"
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "variables.%", "2"),
					func(s *terraform.State) error {
						envID := s.RootModule().Resources[addr].Primary.Attributes["environment_id"]
						return testAccClient().DeleteEnvironmentVariables(context.Background(), envID,
							client.DeleteEnvironmentVariablesRequest{Keys: []string{"TFACC_TWO"}})
					},
				),
				// The refresh after apply sees TFACC_TWO gone and plans to
				// put it back; that is drift detection working.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccEnvironmentVariablesConfig(name, vars string) string {
	return fmt.Sprintf(`
resource "laravel_cloud_application" "test" {
  name       = %[1]q
  repository = %[2]q
  region     = "us-east-2"
}

# Not "production": the API refuses to delete an application's production
# environment.
resource "laravel_cloud_environment" "test" {
  application_id = laravel_cloud_application.test.id
  name           = "staging"
  branch         = %[3]q
}

resource "laravel_cloud_environment_variables" "test" {
  environment_id = laravel_cloud_environment.test.id
  variables = {%[4]s}
}
`, name, testAccRepository(), testAccBranch(), vars)
}
