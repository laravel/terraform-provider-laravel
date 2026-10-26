package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Write-only arguments exist from Terraform 1.11; older versions reject them.
var requireWriteOnly = []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)}

func secretWOConfig(baseURL, value string, version int) string {
	return providerCfg(baseURL) + fmt.Sprintf(`
resource "laravel_cloud_secret" "s" {
  key              = "STRIPE_SECRET"
  value_wo         = %q
  value_wo_version = %d
}
`, value, version)
}

// TestSecretWriteOnlyPlan sends a secret through value_wo. The value reaches
// the API but never state, and is sent again only when its version changes.
func TestSecretWriteOnlyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_secret.s"
	storedValue := func(want string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			rs := s.RootModule().Resources[addr]
			f.mu.Lock()
			defer f.mu.Unlock()
			if got := f.secrets.values[rs.Primary.ID]; got != want {
				return fmt.Errorf("the API holds %q, want %q", got, want)
			}
			// Neither the write-only value nor a copy of it may be in state.
			for _, k := range []string{"value", "value_wo"} {
				if v, ok := rs.Primary.Attributes[k]; ok && v != "" {
					return fmt.Errorf("%s is in state: %q", k, v)
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   requireWriteOnly,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: secretWOConfig(baseURL, "sk_one", 1),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("value_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("value_wo_version"), knownvalue.Int64Exact(1)),
				},
				Check: storedValue("sk_one"),
			},
			{
				// Without a version bump the change cannot be seen, so nothing
				// is sent -- the documented trade-off of a write-only value.
				Config:           secretWOConfig(baseURL, "sk_two", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            storedValue("sk_one"),
			},
			{
				Config: secretWOConfig(baseURL, "sk_two", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				Check: storedValue("sk_two"),
			},
			{
				Config:           secretWOConfig(baseURL, "sk_two", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestEnvironmentNightwatchTokenWriteOnlyPlan connects Nightwatch with the
// write-only token, rotates it with a version bump, and disconnects it by
// removing both.
func TestEnvironmentNightwatchTokenWriteOnlyPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment.env"
	const rotated = "nw_fffffffffffffffffffffffffffffffffffffffff"
	withToken := func(token string, version int) string {
		return environmentSettingsConfig(baseURL, fmt.Sprintf(`
  nightwatch_token_wo         = %q
  nightwatch_token_wo_version = %d
`, token, version))
	}
	sentToken := func(want any) resource.TestCheckFunc {
		return func(*terraform.State) error {
			body, err := lastEnvUpdate(f)
			if err != nil {
				return err
			}
			if got, ok := body["nightwatch_token"]; !ok || got != want {
				return fmt.Errorf("nightwatch_token sent %v (present: %v), want %v", got, ok, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   requireWriteOnly,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: withToken(testNightwatchToken, 1),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("nightwatch_token_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("nightwatch_token"), knownvalue.Null()),
				},
				Check: sentToken(testNightwatchToken),
			},
			{
				Config:           withToken(testNightwatchToken, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: withToken(rotated, 2),
				Check:  sentToken(rotated),
			},
			{
				Config: environmentSettingsConfig(baseURL, ""),
				Check:  sentToken(nil),
			},
			{
				Config:           environmentSettingsConfig(baseURL, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
