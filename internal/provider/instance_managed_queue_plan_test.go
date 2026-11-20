package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestInstanceManagedQueuePlan: a managed queue must send the
// worker entry the API requires, so both generations can be created: a
// current-generation queue with no min_replicas, and a previous-generation
// queue, which needs polling_interval and can change it in place.
func TestInstanceManagedQueuePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const current, legacy = "laravel_cloud_instance.current", "laravel_cloud_instance.legacy"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: managedQueuesConfig(baseURL, 5),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(current, tfjsonpath.New("polling_interval"), knownvalue.Null()),
					statecheck.ExpectKnownValue(legacy, tfjsonpath.New("polling_interval"), knownvalue.Int64Exact(5)),
				},
			},
			{
				Config:           managedQueuesConfig(baseURL, 5),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: managedQueuesConfig(baseURL, 10),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(legacy, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(current, plancheck.ResourceActionNoop),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(legacy, tfjsonpath.New("polling_interval"), knownvalue.Int64Exact(10)),
				},
			},
		},
	})
}

// TestInstanceFifoQueuePlan: a FIFO queue's name comes back
// with the ".fifo" suffix the API adds, which must not show as a diff, and
// fifo must survive an import, where only queue_type tells it apart.
func TestInstanceFifoQueuePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const fifo, standard = "laravel_cloud_instance.fifo", "laravel_cloud_instance.standard"
	config := func(isFifo bool) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "fifo" {
  environment_id = laravel_cloud_environment.env.id
  name           = "orders"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
  fifo           = %t
}

resource "laravel_cloud_instance" "standard" {
  environment_id = laravel_cloud_environment.env.id
  name           = "jobs"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
}
`, isFifo)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(fifo, tfjsonpath.New("name"), knownvalue.StringExact("orders")),
					statecheck.ExpectKnownValue(fifo, tfjsonpath.New("fifo"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(fifo, tfjsonpath.New("queue_type"), knownvalue.StringExact("fifo")),
					statecheck.ExpectKnownValue(standard, tfjsonpath.New("fifo"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(standard, tfjsonpath.New("queue_type"), knownvalue.StringExact("standard")),
					statecheck.ExpectKnownValue(standard, tfjsonpath.New("retired_at"), knownvalue.Null()),
				},
			},
			{
				Config:           config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{ResourceName: fifo, ImportState: true, ImportStateVerify: true},
			{
				// The API cannot change a queue's type in place.
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(fifo, plancheck.ResourceActionReplace),
					plancheck.ExpectResourceAction(standard, plancheck.ResourceActionNoop),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(fifo, tfjsonpath.New("queue_type"), knownvalue.StringExact("standard")),
				},
			},
		},
	})
}

// TestInstanceRenamePlan: the API renames a managed queue's
// instance but not its SQS queue, so renaming a queue must replace it, while
// any other instance still renames in place.
func TestInstanceRenamePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const queue, service = "laravel_cloud_instance.queue", "laravel_cloud_instance.service"
	config := func(suffix string) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "queue" {
  environment_id = laravel_cloud_environment.env.id
  name           = "jobs%[1]s"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
}

resource "laravel_cloud_instance" "service" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web%[1]s"
  size           = "flex-512mb"
  scaling_type   = "none"
}
`, suffix)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("")},
			{
				Config: config("-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(queue, plancheck.ResourceActionReplace),
					plancheck.ExpectResourceAction(service, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(queue, tfjsonpath.New("name"), knownvalue.StringExact("jobs-renamed")),
					statecheck.ExpectKnownValue(service, tfjsonpath.New("name"), knownvalue.StringExact("web-renamed")),
				},
			},
			{
				Config:           config("-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestInstanceManagedQueueValidationPlan rejects settings the API ignores or
// refuses for a queue's generation.
func TestInstanceManagedQueueValidationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	queue := func(size, extra string) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "q" {
  environment_id = laravel_cloud_environment.env.id
  name           = "jobs"
  type           = "managed_queue"
  size           = %q
  scaling_type   = "custom"
  max_replicas   = 2
  %s
}
`, size, extra)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: queue("mq.flex.256mb", "polling_interval = 5"), ExpectError: regexp.MustCompile(`polling_interval does not apply`)},
			{Config: queue("mq-pro-256mb", "polling_interval = 5\n  min_replicas = 0"), ExpectError: regexp.MustCompile(`min_replicas does not apply to this queue`)},
			{Config: strings.Replace(queue("mq.flex.256mb", "fifo = true"), `"jobs"`, `"`+strings.Repeat("j", 35)+`"`, 1), ExpectError: regexp.MustCompile(`name is too long for a FIFO queue`)},
			{Config: strings.Replace(queue("flex-512mb", "fifo = true"), `"managed_queue"`, `"service"`, 1), ExpectError: regexp.MustCompile(`fifo is only for managed queues`)},
			{Config: strings.Replace(queue("flex-512mb", "paused = true"), `"managed_queue"`, `"service"`, 1), ExpectError: regexp.MustCompile(`paused is only for managed queues`)},
		},
	})
}

func managedQueuesConfig(baseURL string, pollingInterval int) string {
	return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "current" {
  environment_id = laravel_cloud_environment.env.id
  name           = "current"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
}

resource "laravel_cloud_instance" "legacy" {
  environment_id   = laravel_cloud_environment.env.id
  name             = "legacy"
  type             = "managed_queue"
  size             = "mq-pro-256mb"
  scaling_type     = "custom"
  max_replicas     = 2
  polling_interval = %d
}
`, pollingInterval)
}

// TestInstanceQueuePausedDefaultPlan: a queue created with
// is_default = true or paused = true must get there in the same apply,
// although the API refuses both while the new queue is still creating, and
// paused must then pause and resume the queue in place.
func TestInstanceQueuePausedDefaultPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	const first, second = "laravel_cloud_instance.first", "laravel_cloud_instance.second"
	config := func(paused string) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
# The environment's first queue, which Cloud makes the default.
resource "laravel_cloud_instance" "first" {
  environment_id = laravel_cloud_environment.env.id
  name           = "first"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
}

resource "laravel_cloud_instance" "second" {
  environment_id = laravel_cloud_environment.env.id
  name           = "second"
  type           = "managed_queue"
  size           = "mq.flex.256mb"
  scaling_type   = "custom"
  max_replicas   = 2
  is_default     = true
  %s
  depends_on     = [laravel_cloud_instance.first]
}
`, paused)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("paused = true"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(second, tfjsonpath.New("is_default"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(second, tfjsonpath.New("paused"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(first, tfjsonpath.New("paused"), knownvalue.Bool(false)),
				},
			},
			{
				Config:           config("paused = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(first, tfjsonpath.New("is_default"), knownvalue.Bool(false)),
				},
			},
			{
				Config: config("paused = false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(second, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(second, tfjsonpath.New("paused"), knownvalue.Bool(false)),
				},
			},
			{
				// Leaving paused out keeps the queue as it is.
				Config:           config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{ResourceName: second, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"is_default"}},
		},
	})
}
