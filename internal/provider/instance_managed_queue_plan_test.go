package provider

import (
	"fmt"
	"regexp"
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
