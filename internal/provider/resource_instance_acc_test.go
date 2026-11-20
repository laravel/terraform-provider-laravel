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

// An automatically scaled instance must apply and converge, and its
// CPU and memory thresholds must be changeable and clearable. The API reports
// replica counts for "auto" (1 up to the plan's limit), which the provider
// used to plan as null, so every create failed with "inconsistent result
// after apply". Thresholds were never sent on update and could not be
// cleared, and the API clears them itself when scaling stops.
func TestAccInstance_autoScaling(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_instance.worker"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccInstanceScalingConfig(name, "auto", `scaling_cpu_threshold_percentage = 70`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "scaling_type", "auto"),
					resource.TestCheckResourceAttr(addr, "min_replicas", "1"),
					resource.TestCheckResourceAttrSet(addr, "max_replicas"),
					resource.TestCheckResourceAttr(addr, "scaling_cpu_threshold_percentage", "70"),
					resource.TestCheckNoResourceAttr(addr, "scaling_memory_threshold_percentage"),
				),
			},
			{
				Config: testAccInstanceScalingConfig(name, "auto",
					"scaling_cpu_threshold_percentage = 80\n  scaling_memory_threshold_percentage = 60"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "scaling_cpu_threshold_percentage", "80"),
					resource.TestCheckResourceAttr(addr, "scaling_memory_threshold_percentage", "60"),
					testAccCheckInstanceThresholds(addr, intPtr(80), intPtr(60)),
				),
			},
			{
				Config: testAccInstanceScalingConfig(name, "auto", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(addr, "scaling_cpu_threshold_percentage"),
					resource.TestCheckNoResourceAttr(addr, "scaling_memory_threshold_percentage"),
					testAccCheckInstanceThresholds(addr, nil, nil),
				),
			},
			{
				Config: testAccInstanceScalingConfig(name, "custom", "min_replicas = 2\n  max_replicas = 3\n  scaling_cpu_threshold_percentage = 75"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "min_replicas", "2"),
					resource.TestCheckResourceAttr(addr, "max_replicas", "3"),
					resource.TestCheckResourceAttr(addr, "scaling_cpu_threshold_percentage", "75"),
				),
			},
			{
				// Back to auto: the platform picks the replica counts again.
				Config: testAccInstanceScalingConfig(name, "auto", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "min_replicas", "1"),
					resource.TestCheckNoResourceAttr(addr, "scaling_cpu_threshold_percentage"),
					testAccCheckInstanceThresholds(addr, nil, nil),
				),
			},
			{
				// Without scaling the API clears the thresholds and runs one replica.
				Config: testAccInstanceScalingConfig(name, "custom", "min_replicas = 2\n  scaling_cpu_threshold_percentage = 90"),
			},
			{
				Config: testAccInstanceScalingConfig(name, "none", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "min_replicas", "1"),
					resource.TestCheckResourceAttr(addr, "max_replicas", "1"),
					resource.TestCheckNoResourceAttr(addr, "scaling_cpu_threshold_percentage"),
					testAccCheckInstanceThresholds(addr, nil, nil),
				),
			},
		},
	})
}

func intPtr(v int) *int { return &v }

// testAccCheckInstanceThresholds reads the instance from the API, so a
// threshold the provider kept in state but never sent is caught.
func testAccCheckInstanceThresholds(addr string, cpu, memory *int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		inst, err := testAccClient().GetInstance(context.Background(), s.RootModule().Resources[addr].Primary.ID)
		if err != nil {
			return err
		}
		for _, c := range []struct {
			name      string
			want, got *int
		}{
			{"scaling_cpu_threshold_percentage", cpu, inst.Attributes.ScalingCPUThresholdPercentage},
			{"scaling_memory_threshold_percentage", memory, inst.Attributes.ScalingMemoryThresholdPercentage},
		} {
			if (c.want == nil) != (c.got == nil) || (c.want != nil && *c.want != *c.got) {
				return fmt.Errorf("API %s = %v, want %v", c.name, fmtIntPtr(c.got), fmtIntPtr(c.want))
			}
		}
		return nil
	}
}

func fmtIntPtr(v *int) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprint(*v)
}

func testAccInstanceScalingConfig(name, scalingType, extra string) string {
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

resource "laravel_cloud_instance" "worker" {
  environment_id = laravel_cloud_environment.test.id
  name           = %[1]q
  size           = "flex-1gb"
  scaling_type   = %[4]q
  %[5]s
}
`, name, testAccRepository(), testAccBranch(), scalingType, extra)
}
