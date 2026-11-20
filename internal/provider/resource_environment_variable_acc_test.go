package provider

import (
	"context"
	"fmt"
	"regexp"
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

// TestAccEnvironmentVariables_limit checks that more than 210 variables, the
// most an environment holds, fail at plan time instead of with a 422 on apply.
// It first stores a large set in one request to show sizes below
// the limit still apply. Not 210: a new environment already holds APP_KEY,
// which Cloud sets, and that counts toward the limit.
func TestAccEnvironmentVariables_limit(t *testing.T) {
	name := testAccName()
	addr := "laravel_cloud_environment_variables.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentVariablesConfig(name, numberedVariables(200)),
				Check:  resource.TestCheckResourceAttr(addr, "variables.%", "200"),
			},
			{
				Config:      testAccEnvironmentVariablesConfig(name, numberedVariables(211)),
				ExpectError: regexp.MustCompile(`(?s)at most 210 variables per environment.*sets 211`),
			},
			{
				// The rejected plan changed nothing, and the test's destroy
				// needs a configuration that validates.
				Config: testAccEnvironmentVariablesConfig(name, numberedVariables(200)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
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
