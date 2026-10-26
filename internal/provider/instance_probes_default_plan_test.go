package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func appInstanceConfig(baseURL, probes string) string {
	return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.env.id
  name           = "web"
  type           = "app"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
%s
}
`, probes)
}

// TestInstanceProbesPlan sets health checks on an app instance, re-plans
// empty, notices a check removed outside Terraform, and finally removes the
// attribute, which resets every check.
func TestInstanceProbesPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_instance.app"
	withProbes := appInstanceConfig(baseURL, `
  probes = {
    readiness = {
      path     = "/up"
      interval = 10
    }
    liveness = {
      path  = "/up"
      tries = 5
    }
  }
`)
	without := appInstanceConfig(baseURL, "")

	lastProbes := func() (any, bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		patches := f.patches["instance"]
		v, ok := patches[len(patches)-1]["probes"]
		return v, ok
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Unset: probes stay unmanaged and nothing is sent.
				Config: without,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("probes"), knownvalue.Null()),
				},
			},
			{
				Config: withProbes,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("probes").AtMapKey("readiness").AtMapKey("interval"), knownvalue.Int64Exact(10)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("probes").AtMapKey("startup"), knownvalue.Null()),
				},
			},
			{
				Config:           withProbes,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				// A check removed in the dashboard is drift: put it back.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, attrs := range f.objects["instance"] {
						if p, ok := attrs["probes"].(map[string]any); ok {
							delete(p, "liveness")
						}
					}
				},
				Config: withProbes,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
			},
			{
				Config: without,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("probes"), knownvalue.Null()),
				},
				Check: func(*terraform.State) error {
					v, ok := lastProbes()
					if m, isMap := v.(map[string]any); !ok || !isMap || len(m) != 0 {
						return fmt.Errorf("removing probes must send probes: {}, sent %v (present: %v)", v, ok)
					}
					return nil
				},
			},
			{
				Config:           without,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestInstanceProbesValidationPlan rejects probes the API would drop or ignore, and
// an is_default the API has no way to apply.
func TestInstanceProbesValidationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	queue := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "q" {
  environment_id = laravel_cloud_environment.env.id
  name           = "jobs"
  type           = "managed_queue"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "auto"
  probes = {
    startup = { path = "/up" }
  }
}
`
	emptyCheck := appInstanceConfig(baseURL, `
  probes = {
    startup = {}
  }
`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: queue, ExpectError: regexp.MustCompile(`probes are only for app instances`)},
			{Config: emptyCheck, ExpectError: regexp.MustCompile(`Health check sets no field`)},
			{Config: queuesConfig(baseURL, "is_default = false", ""), ExpectError: regexp.MustCompile(`is_default cannot be set to false`)},
		},
	})
}

func queuesConfig(baseURL, firstDefault, secondDefault string) string {
	return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "first" {
  environment_id = laravel_cloud_environment.env.id
  name           = "first"
  type           = "managed_queue"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "auto"
  %s
}

resource "laravel_cloud_instance" "second" {
  environment_id = laravel_cloud_environment.env.id
  name           = "second"
  type           = "managed_queue"
  size           = "flex.c-1vcpu-256mb"
  scaling_type   = "auto"
  %s
}
`, firstDefault, secondDefault)
}

// TestInstanceDefaultQueuePlan makes one queue the default on create, moves
// the default to the other queue, and checks re-plans stay empty even though
// the API refuses to make the current default the default again.
func TestInstanceDefaultQueuePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	first, second := "laravel_cloud_instance.first", "laravel_cloud_instance.second"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: queuesConfig(baseURL, "is_default = true", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(first, tfjsonpath.New("is_default"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(second, tfjsonpath.New("is_default"), knownvalue.Bool(false)),
				},
			},
			{
				Config:           queuesConfig(baseURL, "is_default = true", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: queuesConfig(baseURL, "", "is_default = true"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(second, tfjsonpath.New("is_default"), knownvalue.Bool(true)),
				},
			},
			{
				// The first queue lost the default on the platform; a refresh
				// picks that up and nothing is left to change.
				Config:           queuesConfig(baseURL, "", "is_default = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(first, tfjsonpath.New("is_default"), knownvalue.Bool(false)),
				},
			},
		},
	})
}
