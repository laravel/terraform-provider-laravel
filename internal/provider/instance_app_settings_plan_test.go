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
