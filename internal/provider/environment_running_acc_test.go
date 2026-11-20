package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// running stops and starts an environment. Starting deploys it, and
// the environment reports itself running only once that deployment succeeds,
// so the apply waits for it and the next plan is empty.
func TestAccEnvironment_running(t *testing.T) {
	name := testAccName()
	const env = "laravel_cloud_environment.test"
	var envID, deploymentID string

	// The first deployment must be done before the environment is stopped:
	// stopping cancels a deployment in progress, or refuses once it is too
	// far along.
	waitForDeployment := func() {
		for start := time.Now(); ; time.Sleep(15 * time.Second) {
			d, err := testAccClient().GetDeployment(context.Background(), deploymentID)
			if err == nil && (d.Attributes.Status == "deployment.succeeded" || strings.HasSuffix(d.Attributes.Status, "failed")) {
				return
			}
			if time.Since(start) > 20*time.Minute {
				t.Fatalf("deployment %s did not finish in 20 minutes (err: %v)", deploymentID, err)
			}
		}
	}
	apiStatus := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			e, err := testAccClient().GetEnvironment(context.Background(), envID)
			if err != nil {
				return err
			}
			if e.Attributes.Status != want {
				return fmt.Errorf("API status = %q, want %q", e.Attributes.Status, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentRunningConfig(name, ""),
				Check: func(s *terraform.State) error {
					envID = s.RootModule().Resources[env].Primary.ID
					deploymentID = s.RootModule().Resources["laravel_cloud_deployment.test"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: waitForDeployment,
				Config:    testAccEnvironmentRunningConfig(name, "running = false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(env, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(env, "running", "false"),
					resource.TestCheckResourceAttr(env, "status", "stopped"),
					apiStatus("stopped"),
				),
			},
			{
				Config: testAccEnvironmentRunningConfig(name, "running = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(env, "running", "true"),
					apiStatus("running"),
				),
			},
			{
				Config:           testAccEnvironmentRunningConfig(name, "running = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func testAccEnvironmentRunningConfig(name, running string) string {
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
  %[4]s
}

# Only a deployed environment can be stopped.
resource "laravel_cloud_deployment" "test" {
  environment_id = laravel_cloud_environment.test.id
}
`, name, testAccRepository(), testAccBranch(), running)
}
