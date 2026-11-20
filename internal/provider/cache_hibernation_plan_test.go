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

func valkeyFlexConfig(baseURL, name, extra string) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_cache" "cache" {
  name                 = %q
  type                 = "laravel_valkey"
  region               = "us-east-2"
  size                 = "valkey-flex-250mb"
  auto_upgrade_enabled = false
  is_public            = false
%s
}
`, name, extra)
}

// TestCacheHibernationPlan is the fast counterpart of
// TestAccCacheResource_hibernation: scale to zero is set on
// create, changed and turned off in place, and eviction_policy is read back.
// The fake refuses the first update after create as busy, and an update that
// turns scale to zero on without its timeout, as the live API does.
func TestCacheHibernationPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_cache.cache"

	lastPatch := func(want map[string]any) resource.TestCheckFunc {
		return func(*terraform.State) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			patches := f.patches["cache"]
			if len(patches) == 0 {
				return fmt.Errorf("no PATCH was sent")
			}
			last := patches[len(patches)-1]
			if fmt.Sprint(last) != fmt.Sprint(want) {
				return fmt.Errorf("PATCH body = %v, want %v", last, want)
			}
			return nil
		}
	}
	hibernation := func(uses bool, timeout knownvalue.Check) []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_hibernation"), knownvalue.Bool(uses)),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), timeout),
			statecheck.ExpectKnownValue(addr, tfjsonpath.New("eviction_policy"), knownvalue.StringExact("allkeys-lru")),
		}
	}
	update := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
	}}
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

	omitted := valkeyFlexConfig(baseURL, "sessions", "")
	on5 := valkeyFlexConfig(baseURL, "sessions", "  uses_hibernation = true\n  hibernation_timeout = 5")
	on10 := valkeyFlexConfig(baseURL, "sessions", "  uses_hibernation = true\n  hibernation_timeout = 10")
	off := valkeyFlexConfig(baseURL, "sessions", "  uses_hibernation = false")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Omitted: the API's defaults are read back.
			{Config: omitted, ConfigStateChecks: hibernation(false, knownvalue.Null())},
			{Config: omitted, ConfigPlanChecks: empty},
			// Turned on right after create, while the cache is still busy.
			{
				Config:            on5,
				ConfigPlanChecks:  update,
				Check:             lastPatch(map[string]any{"uses_hibernation": true, "hibernation_timeout": float64(5)}),
				ConfigStateChecks: hibernation(true, knownvalue.Int64Exact(5)),
			},
			{Config: on5, ConfigPlanChecks: empty},
			// Removing both attributes keeps the cache as it is.
			{Config: omitted, ConfigPlanChecks: empty},
			{
				Config:           on10,
				ConfigPlanChecks: update,
				Check:            lastPatch(map[string]any{"uses_hibernation": true, "hibernation_timeout": float64(10)}),
			},
			// Turning it off sends no timeout and plans the timeout null.
			{
				Config:            off,
				ConfigPlanChecks:  update,
				Check:             lastPatch(map[string]any{"uses_hibernation": false}),
				ConfigStateChecks: hibernation(false, knownvalue.Null()),
			},
			{Config: off, ConfigPlanChecks: empty},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

// TestCacheHibernationCreatePlan covers creating a cache with scale to zero
// on, and the config errors that replace the API's 422s.
func TestCacheHibernationCreatePlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_cache.cache"
	on := valkeyFlexConfig(baseURL, "sessions", "  uses_hibernation = true\n  hibernation_timeout = 5")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      valkeyFlexConfig(baseURL, "sessions", "  uses_hibernation = true"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`hibernation_timeout is required when uses_hibernation is true`),
			},
			{
				Config:      valkeyFlexConfig(baseURL, "sessions", "  hibernation_timeout = 5"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`hibernation_timeout requires uses_hibernation = true`),
			},
			{
				Config: on,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uses_hibernation"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("hibernation_timeout"), knownvalue.Int64Exact(5)),
				},
			},
			{Config: on, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
