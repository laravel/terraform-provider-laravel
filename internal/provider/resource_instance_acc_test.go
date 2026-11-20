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

// A managed queue must be creatable. The API requires one worker
// entry in background_processes, and the provider used to send none, so every
// create failed with a 422 on background_processes.0.type and .processes.
func TestAccInstance_managedQueue(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_instance.queue"
	var queueID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if queueID == "" {
				return nil
			}
			_, err := testAccClient().GetInstance(context.Background(), queueID)
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("queue %s still exists after destroy (err: %v)", queueID, err)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccManagedQueueConfig(name, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "type", "managed_queue"),
					resource.TestCheckResourceAttr(addr, "name", name),
					resource.TestCheckResourceAttr(addr, "min_replicas", "0"),
					resource.TestCheckResourceAttr(addr, "max_replicas", "2"),
					func(s *terraform.State) error {
						queueID = s.RootModule().Resources[addr].Primary.ID
						return nil
					},
				),
			},
			{
				Config: testAccManagedQueueConfig(name, 3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "max_replicas", "3"),
			},
		},
	})
}

func testAccManagedQueueConfig(name string, maxReplicas int) string {
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

resource "laravel_cloud_instance" "queue" {
  environment_id = laravel_cloud_environment.test.id
  name           = %[1]q
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  min_replicas   = 0
  max_replicas   = %[4]d
}
`, name, testAccRepository(), testAccBranch(), maxReplicas)
}
