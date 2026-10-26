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

// TestEnvironmentSecretPlan attaches a secret, re-plans empty, imports the
// attachment, and then detaches the secret behind Terraform's back: the next
// plan must notice and attach it again.
func TestEnvironmentSecretPlan(t *testing.T) {
	f, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment_secret.attach"
	config := providerCfg(baseURL) + appEnvStack + `
resource "laravel_cloud_secret" "s" {
  key   = "API_TOKEN"
  value = "token-value"
}

resource "laravel_cloud_environment_secret" "attach" {
  environment_id = laravel_cloud_environment.env.id
  secret_id      = laravel_cloud_secret.s.id
}
`
	attached := func(want bool) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			rs := s.RootModule().Resources[addr]
			envID, secretID := rs.Primary.Attributes["environment_id"], rs.Primary.Attributes["secret_id"]
			f.mu.Lock()
			defer f.mu.Unlock()
			if got := f.secrets.attached[envID][secretID]; got != want {
				return fmt.Errorf("secret %s attached to %s = %v, want %v", secretID, envID, got, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				Check:            attached(true),
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Detach outside Terraform; refresh must drop it and re-attach.
				PreConfig: func() {
					f.mu.Lock()
					defer f.mu.Unlock()
					for env := range f.secrets.attached {
						f.secrets.attached[env] = map[string]bool{}
					}
				},
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				Check:            attached(true),
			},
		},
	})
}

func resendConfig(baseURL, fromName, keyLine string) string {
	return providerCfg(baseURL) + appEnvStack + fmt.Sprintf(`
resource "laravel_cloud_environment_resend" "mail" {
  environment_id = laravel_cloud_environment.env.id
  from_address   = "hello@example.com"
  from_name      = %q
  %s
}
`, fromName, keyLine)
}

// TestEnvironmentResendPlan attaches Resend with a default key name, changes
// the sender in place, then switches to a named key, which needs a new
// integration because the sending key cannot be changed in place.
func TestEnvironmentResendPlan(t *testing.T) {
	_, baseURL := newFakeCloud(t)
	addr := "laravel_cloud_environment_resend.mail"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:           resendConfig(baseURL, "Example", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{create(addr)}},
				ConfigStateChecks: []statecheck.StateCheck{
					// Unset, it takes the name the platform chose.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("key_name"), knownvalue.StringRegexp(regexp.MustCompile(`^laravel-cloud-env-`))),
				},
			},
			{
				Config:           resendConfig(baseURL, "Example", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: resendConfig(baseURL, "Example Team", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("from_name"), knownvalue.StringExact("Example Team")),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: resendConfig(baseURL, "Example Team", `key_name = "production-mail"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("key_name"), knownvalue.StringExact("production-mail")),
				},
			},
			{
				Config:           resendConfig(baseURL, "Example Team", `key_name = "production-mail"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
