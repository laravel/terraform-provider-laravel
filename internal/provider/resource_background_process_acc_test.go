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

// The API generates a worker's command from its config and rewrites
// the config it stores, so a worker without a configured command failed every
// apply with "inconsistent result after apply", config was never read back,
// and changing type replaced the process although the API changes it in place.
func TestAccBackgroundProcess_worker(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_background_process.test"
	var processID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if processID == "" {
				return nil
			}
			_, err := testAccClient().GetBackgroundProcess(context.Background(), processID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("background process %s still exists after destroy (err: %v)", processID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccBackgroundProcessWorkerConfig(name, 1, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "type", "worker"),
					resource.TestMatchResourceAttr(addr, "command", regexp.MustCompile(`queue:work .*--tries=3`)),
					func(s *terraform.State) error {
						processID = s.RootModule().Resources[addr].Primary.ID
						return nil
					},
				),
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// A config change made outside Terraform shows up as drift.
				PreConfig: func() {
					_, err := testAccClient().UpdateBackgroundProcess(context.Background(), processID, client.UpdateBackgroundProcessRequest{
						Config: map[string]any{"queue": "default", "tries": 7},
					})
					if err != nil {
						t.Fatalf("changing config out of band: %v", err)
					}
				},
				Config: testAccBackgroundProcessWorkerConfig(name, 1, 3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestMatchResourceAttr(addr, "command", regexp.MustCompile(`--tries=3`)),
			},
			{
				Config: testAccBackgroundProcessWorkerConfig(name, 2, 5),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "processes", "2"),
					resource.TestMatchResourceAttr(addr, "command", regexp.MustCompile(`--tries=5`)),
				),
			},
			{
				Config: testAccBackgroundProcessCustomConfig(name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "type", "custom"),
					resource.TestCheckResourceAttr(addr, "command", "php artisan schedule:work"),
					resource.TestCheckResourceAttrPtr(addr, "id", &processID),
				),
			},
		},
	})
}

func testAccBackgroundProcessStack(name string) string {
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

resource "laravel_cloud_instance" "test" {
  environment_id = laravel_cloud_environment.test.id
  name           = %[1]q
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
  # The API refuses to delete the last background process of a service whose
  # scheduler is off, so destroy could not remove the worker.
  uses_scheduler = true
}
`, name, testAccRepository(), testAccBranch())
}

func testAccBackgroundProcessWorkerConfig(name string, processes, tries int) string {
	return testAccBackgroundProcessStack(name) + fmt.Sprintf(`
resource "laravel_cloud_background_process" "test" {
  instance_id = laravel_cloud_instance.test.id
  type        = "worker"
  processes   = %d
  config      = jsonencode({ queue = "default", tries = %d })
}
`, processes, tries)
}

func testAccBackgroundProcessCustomConfig(name string) string {
	return testAccBackgroundProcessStack(name) + `
resource "laravel_cloud_background_process" "test" {
  instance_id = laravel_cloud_instance.test.id
  type        = "custom"
  processes   = 1
  command     = "php artisan schedule:work"
}
`
}
