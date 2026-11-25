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

// TestInstanceAppSettingsPlan covers the settings only an app instance has:
// uses_octane, uses_inertia_ssr and hibernation_timeout, in minutes. They are
// read back, so a change made outside Terraform shows in the plan, and 0 turns
// hibernation off, which the API spells null.
func TestInstanceAppSettingsPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_instance.app"

	lastPatch := func() map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		patches := f.patches["instance"]
		return patches[len(patches)-1]
	}
	expectPatch := func(key string, want any) resource.TestCheckFunc {
		return func(*terraform.State) error {
			got, ok := lastPatch()[key]
			if !ok || got != want {
				return fmt.Errorf("last PATCH has %s = %v (sent: %v), want %v", key, got, ok, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Unset: the platform's values are read back.
				Config: appInstanceConfig(baseURL, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_octane"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_inertia_ssr"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), knownvalue.Int64Exact(5)),
				},
			},
			{
				Config: appInstanceConfig(baseURL, "uses_octane = true\n  hibernation_timeout = 10"),
				Check: resource.ComposeAggregateTestCheckFunc(
					expectPatch("uses_octane", true),
					expectPatch("hibernation_timeout", float64(10)),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), knownvalue.Int64Exact(10)),
				},
			},
			{
				// Changed outside Terraform: the plan puts it back.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, a := range f.objects["instance"] {
						a["hibernation_timeout"] = 30
					}
				},
				Config: appInstanceConfig(baseURL, "uses_octane = true\n  hibernation_timeout = 10"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: expectPatch("hibernation_timeout", float64(10)),
			},
			{
				// 0 turns hibernation off: sent as null, read back as 0.
				Config: appInstanceConfig(baseURL, "uses_octane = true\n  hibernation_timeout = 0"),
				Check:  expectPatch("hibernation_timeout", nil),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), knownvalue.Int64Exact(0)),
				},
			},
			{
				Config:   appInstanceConfig(baseURL, "uses_octane = true\n  hibernation_timeout = 0"),
				PlanOnly: true,
			},
		},
	})
}

// TestInstanceAppSettingsValidationPlan keeps what the API would reject or
// ignore at plan time: a timeout outside 1-60 minutes (300 is five minutes in
// seconds, which the attribute used to document), and app settings on another
// instance type, which the API drops and reports as null.
func TestInstanceAppSettingsValidationPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)

	service := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_instance" "web" {
  environment_id = laravel_cloud_environment.env.id
  name           = "worker"
  size           = "flex-512mb"
  scaling_type   = "auto"
  uses_octane    = true
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      appInstanceConfig(baseURL, "hibernation_timeout = 300"),
				ExpectError: regexp.MustCompile(`hibernation_timeout is out of range`),
			},
			{
				Config:      service,
				ExpectError: regexp.MustCompile(`uses_octane is only for app instances`),
			},
		},
	})
}

// TestInstanceAdoptAppPlan: type = "app" takes over the app
// instance Laravel Cloud creates with the environment, which the API refuses
// to create, applies the configured settings to it, and leaves it to the
// environment on destroy, as the API refuses to delete it.
func TestInstanceAdoptAppPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_instance.app"
	config := func(settings string) string {
		return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_instance" "app" {
  environment_id = laravel_cloud_environment.env.id
  name           = "App"
  type           = "app"
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
  %s
}
`, settings)
	}
	lastPatch := func() map[string]any {
		f.mu.Lock()
		defer f.mu.Unlock()
		patches := f.patches["instance"]
		if len(patches) == 0 {
			return nil
		}
		return patches[len(patches)-1]
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("uses_octane = true\n  hibernation_timeout = 0"),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						envID := s.RootModule().Resources["laravel_cloud_environment.env"].Primary.ID
						if got, want := s.RootModule().Resources[addr].Primary.ID, "instance-app-"+envID; got != want {
							return fmt.Errorf("id = %q, want the environment's app instance %q", got, want)
						}
						f.mu.Lock()
						defer f.mu.Unlock()
						for id, a := range f.objects["instance"] {
							if a["type"] != "app" {
								return fmt.Errorf("an instance was created: %s %v", id, a)
							}
						}
						return nil
					},
					func(*terraform.State) error {
						p := lastPatch()
						if v, ok := p["hibernation_timeout"]; !ok || v != nil || p["uses_octane"] != true {
							return fmt.Errorf("adopting must PATCH uses_octane = true and hibernation_timeout = null, sent %v", p)
						}
						return nil
					},
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), knownvalue.Int64Exact(0)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_octane"), knownvalue.Bool(true)),
				},
			},
			{
				Config: config("uses_octane = true\n  hibernation_timeout = 15"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: func(*terraform.State) error {
					if v := lastPatch()["hibernation_timeout"]; v != float64(15) {
						return fmt.Errorf("PATCH hibernation_timeout = %v, want 15", v)
					}
					return nil
				},
			},
		},
		// The fake refuses to delete an app instance, so destroy succeeding
		// means none was sent; the instance is still there.
		CheckDestroy: func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			for _, a := range f.objects["instance"] {
				if a["type"] == "app" && a["hibernation_timeout"] == float64(15) {
					return nil
				}
			}
			return fmt.Errorf("destroy deleted the app instance")
		},
	})
}

// TestInstanceAdoptAppMissingPlan: an environment without an app instance
// has nothing to take over, and the API cannot create one.
func TestInstanceAdoptAppMissingPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: providerCfg(baseURL) + appEnvStack},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for id, a := range f.objects["instance"] {
						if a["type"] == "app" {
							delete(f.objects["instance"], id)
						}
					}
				},
				Config:      appInstanceConfig(baseURL, ""),
				ExpectError: regexp.MustCompile(`Environment has no app instance`),
			},
		},
	})
}

// TestInstanceAdoptAppKeepsSchedulerPlan: uses_scheduler defaulted to false,
// so adopting an app instance that runs the scheduler, without setting it,
// turned the scheduler off. Unset, it now keeps what the instance has.
func TestInstanceAdoptAppKeepsSchedulerPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	const addr = "laravel_cloud_instance.app"
	scheduler := func() any {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, a := range f.objects["instance"] {
			if a["type"] == "app" {
				return a["uses_scheduler"]
			}
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: providerCfg(baseURL) + appEnvStack},
			{
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for _, a := range f.objects["instance"] {
						if a["type"] == "app" {
							a["uses_scheduler"] = true
						}
					}
				},
				Config: appInstanceConfig(baseURL, ""),
				Check: func(*terraform.State) error {
					if got := scheduler(); got != true {
						return fmt.Errorf("adopting turned the scheduler off: uses_scheduler = %v", got)
					}
					return nil
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_scheduler"), knownvalue.Bool(true)),
				},
			},
			{
				Config:           appInstanceConfig(baseURL, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
