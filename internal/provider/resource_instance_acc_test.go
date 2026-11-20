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
				Config: testAccManagedQueueConfig(name, 2, ""),
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
				Config: testAccManagedQueueConfig(name, 3, ""),
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

func testAccManagedQueueConfig(name string, maxReplicas int, extra string) string {
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
  %[5]s
}
`, name, testAccRepository(), testAccBranch(), maxReplicas, extra)
}

// A FIFO managed queue must be creatable. The API appends ".fifo" to
// the queue's name, which must read back without it, so the queue converges,
// and fifo must come back on import from queue_type.
func TestAccInstance_fifoQueue(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_instance.queue"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccManagedQueueConfig(name, 2, "fifo = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", name),
					resource.TestCheckResourceAttr(addr, "fifo", "true"),
					resource.TestCheckResourceAttr(addr, "queue_type", "fifo"),
					resource.TestCheckNoResourceAttr(addr, "retired_at"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.ID
						inst, err := testAccClient().GetInstance(context.Background(), id)
						if err != nil {
							return err
						}
						if want := name + ".fifo"; inst.Attributes.Name != want {
							return fmt.Errorf("API name = %q, want %q", inst.Attributes.Name, want)
						}
						return nil
					},
				),
			},
			{
				Config:           testAccManagedQueueConfig(name, 2, "fifo = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// queue_status can move on between the apply and the import.
			{ResourceName: addr, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"queue_status"}},
		},
	})
}

// paused pauses and resumes a managed queue, and a queue created with
// is_default = true takes the default over in the same apply, although the
// API refuses both until the new queue is available.
func TestAccInstance_pausedDefaultQueue(t *testing.T) {
	name := testAccName()
	const queue, second = "laravel_cloud_instance.queue", "laravel_cloud_instance.second"
	config := func(paused bool) string {
		// The helper's queue is the environment's first, which Cloud makes
		// the default.
		return testAccManagedQueueConfig(name, 2, fmt.Sprintf("paused = %t", paused)) + fmt.Sprintf(`
resource "laravel_cloud_instance" "second" {
  environment_id = laravel_cloud_environment.test.id
  name           = "%[1]s-b"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  min_replicas   = 0
  max_replicas   = 2
  is_default     = true
  depends_on     = [laravel_cloud_instance.queue]
}
`, name)
	}
	apiState := func(addr string, wantPaused, wantDefault bool) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			inst, err := testAccClient().GetInstance(context.Background(), s.RootModule().Resources[addr].Primary.ID)
			if err != nil {
				return err
			}
			if p := inst.Attributes.Paused; p == nil || *p != wantPaused {
				return fmt.Errorf("%s: API paused = %v, want %t", addr, p, wantPaused)
			}
			if d := inst.Attributes.IsDefault; d == nil || *d != wantDefault {
				return fmt.Errorf("%s: API is_default = %v, want %t", addr, d, wantDefault)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(queue, "paused", "true"),
					resource.TestCheckResourceAttr(second, "is_default", "true"),
					apiState(queue, true, false),
					apiState(second, false, true),
				),
			},
			{
				Config:           config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(queue, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(queue, "paused", "false"),
					apiState(queue, false, false),
				),
			},
			{
				Config:           config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
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

// An environment's app instance, imported, reads back its Octane,
// Inertia SSR and hibernation settings, takes hibernation_timeout in minutes,
// and turns hibernation off with 0. The provider used to read none of them
// back, documented the timeout in seconds, and never sent null, so
// hibernation could not be turned off. Destroy leaves the app instance to the
// environment, as the API refuses to delete it.
func TestAccInstance_appSettings(t *testing.T) {
	name := testAccName()
	const addr = "laravel_cloud_instance.app"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccAppInstanceConfig(name, "", false)},
			{
				Config:             testAccAppInstanceConfig(name, "", true),
				ResourceName:       addr,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					envID := s.RootModule().Resources["laravel_cloud_environment.test"].Primary.ID
					instances, err := testAccClient().ListInstances(context.Background(), envID)
					if err != nil {
						return "", err
					}
					for _, i := range instances {
						if i.Attributes.InstanceType == "app" {
							return i.ID, nil
						}
					}
					return "", fmt.Errorf("environment %s has no app instance", envID)
				},
			},
			{
				// A new environment's app instance hibernates after 5 minutes.
				Config: testAccAppInstanceConfig(name, "", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "uses_octane", "false"),
					resource.TestCheckResourceAttr(addr, "uses_inertia_ssr", "false"),
					resource.TestCheckResourceAttr(addr, "hibernation_timeout", "5"),
				),
			},
			{
				Config: testAccAppInstanceConfig(name, "uses_octane = true\n  hibernation_timeout = 10", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "uses_octane", "true"),
					resource.TestCheckResourceAttr(addr, "hibernation_timeout", "10"),
					testAccCheckAppInstance(addr, true, intPtr(10)),
				),
			},
			{
				Config: testAccAppInstanceConfig(name, "uses_octane = true\n  hibernation_timeout = 0", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "hibernation_timeout", "0"),
					testAccCheckAppInstance(addr, true, nil),
				),
			},
		},
	})
}

// testAccCheckAppInstance reads the app instance's settings from the API.
func testAccCheckAppInstance(addr string, octane bool, hibernation *int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		inst, err := testAccClient().GetInstance(context.Background(), s.RootModule().Resources[addr].Primary.ID)
		if err != nil {
			return err
		}
		a := inst.Attributes
		if a.UsesOctane == nil || *a.UsesOctane != octane {
			return fmt.Errorf("API uses_octane = %v, want %v", a.UsesOctane, octane)
		}
		var got *int
		if a.HibernationTimeout != nil {
			got = intPtr(int(*a.HibernationTimeout))
		}
		if (got == nil) != (hibernation == nil) || (got != nil && *got != *hibernation) {
			return fmt.Errorf("API hibernation_timeout = %s, want %s", fmtIntPtr(got), fmtIntPtr(hibernation))
		}
		return nil
	}
}

func testAccAppInstanceConfig(name, extra string, withInstance bool) string {
	cfg := fmt.Sprintf(`
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
`, name, testAccRepository(), testAccBranch())
	if !withInstance {
		return cfg
	}
	// Laravel Cloud creates the app instance with the environment, named App.
	return cfg + fmt.Sprintf(`
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.test.id
  name           = "App"
  type           = "app"
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
  %s
}
`, extra)
}
